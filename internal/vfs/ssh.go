package vfs

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHFS struct {
	sftpBase
}

func NewSSHFS(sshClient *ssh.Client, sftpClient *sftp.Client, dir string) (*SSHFS, error) {
	return &SSHFS{
		sftpBase: sftpBase{
			BaseDir:    dir,
			sftpClient: sftpClient,
			sshClient:  sshClient,
		},
	}, nil
}

// Use remote shell commands to compute hash on the server, avodiing streaming the file over SFTP just to compute the hash.
func (fs *SSHFS) HashMod(filename string) (string, error) {
	path := filepath.ToSlash(filepath.Join(fs.BaseDir, filename))
	// TODO: Decide if we should check for the command
	output, err := fs.runSafeCmd(10*time.Second, "sha1sum", path)
	if err != nil {
		return "", fmt.Errorf("failed to compute hash on server: %w", err)
	}

	parts := strings.Fields(string(output))
	if len(parts) == 0 {
		return "", fmt.Errorf("unexpected output from sha1sum: %s", output)
	}
	return parts[0], nil
}

func (fs *SSHFS) HashMods() (map[string]string, error) {
	hashes := make(map[string]string)

	mods, err := fs.ListMods()
	if err != nil {
		return nil, fmt.Errorf("failed to list mods: %w", err)
	}

	for _, mod := range mods {
		hash, err := fs.HashMod(mod)
		if err != nil {
			return nil, fmt.Errorf("failed to hash mod %s: %w", mod, err)
		}
		hashes[mod] = hash
	}

	return hashes, nil
}

func (fs *SSHFS) DownloadMod(url string, filename string) error {
	downloadMethod := fs.determineDownloadMethod(fs.sshClient)
	// Attempt serverside download
	switch downloadMethod {
	case "curl":
		_, err := fs.runSafeCmd(30*time.Second, "curl", "-L", "-o", filename, url)
		if err != nil {
			return fmt.Errorf("failed to download mod with curl: %w", err)
		}

	case "wget":
		_, err := fs.runSafeCmd(30*time.Second, "wget", "-O", filename, url)
		if err != nil {
			return fmt.Errorf("failed to download mod with wget: %w", err)
		}
	default:
		// Some kind of error
		return fmt.Errorf("no download tool available on server (curl or wget), cannot download mod")
	}
	return nil
}

func (fs *SSHFS) CleanupTmpFiles() error {
	target := fs.BaseDir + "/*.gorinth-tmp"
	_, err := fs.runSafeCmd(30*time.Second, "rm", "-f", target)
	return err
}

func (fs *SSHFS) Backup(baseDirName string) (string, error) {
	timestamp := time.Now().Format("2006-01-02_150405")
	parentDir := filepath.Dir(fs.BaseDir)
	tarFileName := fmt.Sprintf("%s_backup_%s.tar", baseDirName, timestamp)
	tarPath := filepath.ToSlash(filepath.Join(parentDir, tarFileName))

	_, err := fs.runSafeCmd(30*time.Second, "tar", "-cf", tarPath, "-C", parentDir, baseDirName)
	if err != nil {
		return "", fmt.Errorf("failed to create backup tarball: %w", err)
	}

	return tarPath, nil

}

func (fs *SSHFS) determineDownloadMethod(client *ssh.Client) string {
	_, err := fs.runSafeCmd(2*time.Second, "command", "-v", "curl")
	if err == nil {
		return "curl"
	}

	_, err = fs.runSafeCmd(2*time.Second, "command", "-v", "wget")
	if err == nil {
		return "wget"
	}

	return "sftp" // fallback to sftp method
}

// runSafeCmd executes a command on the SSH server with a timeout, ensuring that the session is properly closed if the command takes too long. It also escapes all arguments to prevent injection issues.
func (fs *SSHFS) runSafeCmd(timeout time.Duration, cmd string, args ...string) (string, error) {
	// Construct a safe command by escaping all arguments
	var safeCmdBuilder strings.Builder
	safeCmdBuilder.WriteString(cmd)
	for _, arg := range args {
		safeCmdBuilder.WriteString(" " + escapeArg(arg))
	}
	finalCmd := safeCmdBuilder.String()

	// Timeout on session creation
	sessionCh := make(chan *ssh.Session, 1)
	errCh := make(chan error, 1)

	go func() {
		s, err := fs.sshClient.NewSession()
		if err != nil {
			errCh <- err
			return
		}
		sessionCh <- s
	}()

	var session *ssh.Session
	select {
	case <-time.After(timeout):
		return "", fmt.Errorf("timed out waiting for server to create SSH session")
	case err := <-errCh:
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	case session = <-sessionCh:
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	// Timeout wrapper for command execution
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- session.Run(finalCmd)
	}()

	select {
	case <-time.After(timeout):
		session.Close()
		<-doneCh // Ensure goroutine exits
		return "", fmt.Errorf("command timed out and session was closed")
	case err := <-doneCh:
		if err != nil {
			return "", fmt.Errorf("command execution failed: %w", err)
		}
		return stdoutBuf.String(), nil
	}

}

func escapeArg(arg string) string {
	if arg == "" {
		return "''"
	}

	escaped := strings.ReplaceAll(arg, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

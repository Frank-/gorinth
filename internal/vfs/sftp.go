package vfs

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPFS struct {
	BaseDir      string
	sshClient    *ssh.Client
	sftpClient   *sftp.Client
	hasShell     bool
	downloadTool string // "curl" or "wget" or "sftp" fallback
}

// Create a new SSH connection and init SFTP client
func NewSFTPFS(host string, port int, user, pass, dir string) (*SFTPFS, error) {
	tui.Logger.Debug("Initializing SFTP connection", "host", host, "port", port, "user", user, "dir", dir)
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	tui.Logger.Debug("SSH connection established successfully")
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	tui.Logger.Debug("SFTP client initialized successfully")
	// Verify that the base directory exists and is a directory
	info, err := sftpClient.Stat(dir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("remote mod directory does not exist: %s", dir)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to access remote mod directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("remote mod path is not a directory: %s", dir)
	}

	tui.Logger.Debug("SFTP connection established successfully")

	tui.Logger.Debug("Probing for SSH shell access to determine available operations")
	// Check for shell access to determine if we can do server-side operations like direct downloads and tar backups
	hasShell := shellProbe(sshClient)
	if hasShell {
		tui.Logger.Debug("SSH shell access confirmed, server-side operations enabled")
	} else {
		tui.Logger.Debug("No SSH shell access, falling back to SFTP-only operations")
	}

	return &SFTPFS{
		BaseDir:    dir,
		sshClient:  sshClient,
		sftpClient: sftpClient,
		hasShell:   hasShell,
	}, nil
}

func (fs *SFTPFS) ListMods() ([]string, error) {
	var mods []string
	entries, err := fs.sftpClient.ReadDir(fs.BaseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// We only care about .jar files in the mods directory
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jar") {
			mods = append(mods, entry.Name())
		}
	}
	return mods, nil
}

// Stream the file from SFTP and compute its SHA1 hash without loading the entire file into memory
func (fs *SFTPFS) HashMod(filename string) (string, error) {
	path := filepath.ToSlash(filepath.Join(fs.BaseDir, filename))
	file, err := fs.sftpClient.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (fs *SFTPFS) HashMods() (map[string]string, error) {
	hashes := make(map[string]string)

	mods, err := fs.ListMods()
	if err != nil {
		return nil, fmt.Errorf("failed to list mods: %w", err)
	}

	for _, mod := range mods {
		hash, err := fs.HashMod(mod)
		if err != nil {
			return nil, fmt.Errorf("failed to hash mod '%s': %w", mod, err)
		}
		hashes[mod] = hash
	}

	return hashes, nil
}

func (fs *SFTPFS) DeleteMod(filename string) error {
	path := filepath.ToSlash(filepath.Join(fs.BaseDir, filename))
	return fs.sftpClient.Remove(path)
}

func (fs *SFTPFS) WriteMod(filename string, data io.Reader) error {
	path := filepath.ToSlash(filepath.Join(fs.BaseDir, filename))
	file, err := fs.sftpClient.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	return err
}

func (fs *SFTPFS) Rename(oldName, newName string) error {
	oldPath := filepath.ToSlash(filepath.Join(fs.BaseDir, oldName))
	newPath := filepath.ToSlash(filepath.Join(fs.BaseDir, newName))
	return fs.sftpClient.Rename(oldPath, newPath)
}

func (fs *SFTPFS) DownloadMod(url string, targetFilename string) error {
	destPath := filepath.ToSlash(filepath.Join(fs.BaseDir, targetFilename))

	if fs.downloadTool == "" {
		// fs.downloadTool = "sftp" // default to sftp fallback

		// if session, err := fs.sshClient.NewSession(); err == nil {
		// 	// check for curl
		// 	if err := session.Run("command -v curl"); err == nil {
		// 		fs.downloadTool = "curl"
		// 	}
		// 	session.Close()
		// }

		// if fs.downloadTool == "sftp" {
		// 	if session, err := fs.sshClient.NewSession(); err == nil {
		// 		// check for wget
		// 		if err := session.Run("command -v wget"); err == nil {
		// 			fs.downloadTool = "wget"
		// 		}
		// 		session.Close()
		// 	}
		// }

		fs.downloadTool = fs.determineDownloadMethod(fs.sshClient)

		tui.Logger.Debug("Selected download method", "method", fs.downloadTool)
	}

	// Attempt serverside download
	if fs.downloadTool == "curl" || fs.downloadTool == "wget" {
		var cmd string
		if fs.downloadTool == "curl" {
			cmd = fmt.Sprintf("curl -sL -o '%s' '%s' >/dev/null 2>&1", destPath, url)
		} else {
			cmd = fmt.Sprintf("wget -q -O '%s' '%s' >/dev/null 2>&1", destPath, url)
		}

		_, err := runCmdWithTimeout(fs.sshClient, cmd, 60*time.Second)
		if err == nil {
			tui.Logger.Debug("Server-side download successful", "url", url, "dest", destPath)
			return nil
		}
		tui.Logger.Warn("Server-side download failed, falling back to local download and upload", "error", err)
		// 	if session, err := fs.sshClient.NewSession(); err == nil {
		// 		cmd := fmt.Sprintf("curl -sL -o '%s' '%s' >/dev/null 2>&1", destPath, url)
		// 		err = session.Run(cmd)
		// 		session.Close()
		// 		if err == nil {
		// 			return nil // Download succeeded
		// 		}
		// 	}
		// }

		// if fs.downloadTool == "wget" {
		// 	if session, err := fs.sshClient.NewSession(); err == nil {
		// 		cmd := fmt.Sprintf("wget -q -O '%s' '%s' >/dev/null 2>&1", destPath, url)
		// 		err = session.Run(cmd)
		// 		session.Close()
		// 		if err == nil {
		// 			return nil // Download succeeded with wget
		// 		}
		// 	}
	}

	// If serverside download failed, fall back to local download and upload
	tui.Logger.Debug("Downloading file locally via HTTP...")
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download mod: %w", err)
	}
	defer resp.Body.Close()

	destFile, err := fs.sftpClient.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file for mod: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, resp.Body)
	return err
}

func (fs *SFTPFS) SyncToDir(dest string) error {
	mods, err := fs.ListMods()
	if err != nil {
		return fmt.Errorf("failed to list mods for sync: %w", err)
	}

	for _, mod := range mods {
		srcPath := filepath.ToSlash(filepath.Join(fs.BaseDir, mod))
		destPath := filepath.Join(dest, mod)

		srcFile, err := fs.sftpClient.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open remote mod file: %w", err)
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to create local file for sync: %w", err)
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			return fmt.Errorf("failed to copy mod file during sync: %w", err)
		}
	}

	return nil
}

func (fs *SFTPFS) Backup(baseDirName string) (string, error) {
	timestamp := time.Now().Format("2006-01-02_150405")

	/*
	* Server-side backup using tar (if shell access is available)
	 */

	if fs.hasShell {
		tui.Logger.Debug("Attempting server-side backup using tar", "baseDir", fs.BaseDir)

		parentDir := filepath.Dir(fs.BaseDir)
		tarFileName := fmt.Sprintf("%s_backup_%s.tar", baseDirName, timestamp)
		tarPath := filepath.ToSlash(filepath.Join(parentDir, tarFileName))

		// if session, err := fs.sshClient.NewSession(); err == nil {
		cmd := fmt.Sprintf("tar -cf '%s' -C '%s' '%s' >/dev/null 2>&1", tarPath, parentDir, baseDirName)
		_, err := runCmdWithTimeout(fs.sshClient, cmd, 30*time.Second)
		if err == nil {
			tui.Logger.Debug("Server-side tar backup successful", "tarPath", tarPath)
			return tarPath, nil
		}
		tui.Logger.Warn("Server-side tar backup failed, falling back to zip method", "error", err)
	}

	/*
	* Local zip backup
	 */

	mods, err := fs.ListMods()
	if err != nil {
		return "", fmt.Errorf("failed to list mods for backup: %w", err)
	}

	tui.Logger.Debug("Creating local zip backup of mods", "modCount", len(mods))
	return createLocalZip(baseDirName, mods, func(mod string) (io.ReadCloser, error) {
		path := filepath.ToSlash(filepath.Join(fs.BaseDir, mod))
		return fs.sftpClient.Open(path)
	})

}

// Close the SFTP and SSH connections when done
func (fs *SFTPFS) Close() error {
	fs.sftpClient.Close()
	return fs.sshClient.Close()
}

// func shellProbe(sshClient *ssh.Client) bool {
// 	resultCh := make(chan bool, 1)

// 	go func() {
// 		session, err := sshClient.NewSession()
// 		if err != nil {
// 			resultCh <- false
// 			return
// 		}
// 		defer session.Close()

// 		err = session.Run("echo gorinth-probe")
// 		resultCh <- (err == nil)
// 	}()

// 	select {
// 	case <-time.After(3 * time.Second):
// 		tui.Logger.Debug("SSH shell probe timed out, assuming no shell access")
// 		return false // Timeout, assume no shell access
// 	case res := <-resultCh:
// 		return res
// 	}

// }

func shellProbe(client *ssh.Client) bool {
	probeStr := fmt.Sprintf("gorinth-probe-%d", time.Now().Unix())

	out, err := runCmdWithTimeout(client, "echo "+probeStr, 3*time.Second)
	if err != nil {
		tui.Logger.Debug("SSH shell probe failed", "error", err)
		return false
	}

	return strings.Contains(out, probeStr)
}

func (fs *SFTPFS) determineDownloadMethod(client *ssh.Client) string {
	if !fs.hasShell {
		return "sftp" // no shell access, must use sftp
	}

	_, err := runCmdWithTimeout(client, "command -v curl || which curl", 2*time.Second)
	if err == nil {
		return "curl"
	}

	_, err = runCmdWithTimeout(client, "command -v wget || which wget", 2*time.Second)
	if err == nil {
		return "wget"
	}

	return "sftp" // fallback to sftp method
}

// func (fs *SFTPS) runSession(cmd string, timeout time.Duration) error {
// 	session, err := fs.sshClient.NewSession()
// 	if err != nil {
// 		return fmt.Errorf("failed to open session: %w", err)
// 	}

// 	errCh := make(chan error, 1)
// 	go func() {
// 		errCh <- session.Run(cmd)
// 	}()

// 	select {
// 	case <-time.After(timeout):

// 		session.Close()
// 		return fmt.Errorf("command timed out and session was closed")
// 	case err := <-errCh:
// 		session.Close()
// 		return err
// 	}
// }

// func runWithTimeout(session *ssh.Session, command string, timeout time.Duration) error {
// 	// Create a channel to listen for command completion
// 	errCh := make(chan error, 1)

// 	// Run the command in a separate goroutine
// 	go func() {
// 		errCh <- session.Run(command)
// 	}()

// 	// Run against a timer
// 	select {
// 	case <-time.After(timeout):

// 		// Timeout occurred, attempt to kill the session
// 		_ = session.Signal(ssh.SIGKILL) // Best effort to kill the session
// 		session.Close()
// 		return fmt.Errorf("command timed out and session was closed")
// 	case err := <-errCh:
// 		// Command completed before timeout
// 		return err
// 	}
// }

func runCmdWithTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	tui.Logger.Debug("running command")

	// Timeout wrapper for session creation
	type sessionResult struct {
		session *ssh.Session
		err     error
	}
	sessionCh := make(chan sessionResult, 1)

	go func() {
		s, err := client.NewSession()
		sessionCh <- sessionResult{session: s, err: err}
	}()

	var session *ssh.Session
	select {
	case <-time.After(timeout):
		return "", fmt.Errorf("timed out waiting for server to create SSH session")
	case res := <-sessionCh:
		if res.err != nil {
			return "", fmt.Errorf("failed to create SSH session: %w", res.err)
		}
		session = res.session
	}

	tui.Logger.Debug("session created")

	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	// Timeout wrapper for command execution
	done := make(chan error, 1)
	go func() {
		tui.Logger.Debug("starting command execution", "cmd", cmd)
		done <- session.Run(cmd)
	}()

	select {
	case <-time.After(timeout):
		tui.Logger.Warn("Command timed out, attempting to close session", "cmd", cmd)
		session.Close()
		<-done // Ensure goroutine exits
		return "", fmt.Errorf("command timed out and session was closed")
	case err := <-done:
		tui.Logger.Debug("command execution completed", "cmd", cmd, "output", stdoutBuf.String(), "error", err)
		session.Close()
		if err != nil {
			return "", fmt.Errorf("command execution failed: %w", err)
		}
		return stdoutBuf.String(), nil
	}

}

package vfs

import (
	"archive/zip"
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

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

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
		fs.downloadTool = "sftp" // default to sftp fallback

		if session, err := fs.sshClient.NewSession(); err == nil {
			// check for curl
			if err := session.Run("command -v curl"); err == nil {
				fs.downloadTool = "curl"
			}
			session.Close()
		}

		if fs.downloadTool == "sftp" {
			if session, err := fs.sshClient.NewSession(); err == nil {
				// check for wget
				if err := session.Run("command -v wget"); err == nil {
					fs.downloadTool = "wget"
				}
				session.Close()
			}
		}

		tui.Logger.Debug("Selected download method", "method", fs.downloadTool)
	}

	// Attempt serverside download
	if fs.downloadTool == "curl" {
		if session, err := fs.sshClient.NewSession(); err == nil {
			cmd := fmt.Sprintf("curl -sL -o '%s' '%s' >/dev/null 2>&1", destPath, url)
			err = session.Run(cmd)
			session.Close()
			if err == nil {
				return nil // Download succeeded
			}
		}
	}

	if fs.downloadTool == "wget" {
		if session, err := fs.sshClient.NewSession(); err == nil {
			cmd := fmt.Sprintf("wget -q -O '%s' '%s' >/dev/null 2>&1", destPath, url)
			err = session.Run(cmd)
			session.Close()
			if err == nil {
				return nil // Download succeeded with wget
			}
		}
	}

	// If serverside download failed, fall back to local download and upload
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

func (fs *SFTPFS) Backup() (string, error) {
	timestamp := time.Now().Format("2006-01-02_150405")
	parentDir := filepath.Dir(fs.BaseDir)
	baseDirName := filepath.Base(fs.BaseDir)
	backupDirName := fmt.Sprintf("%s_backup_%s", baseDirName, timestamp)

	tarFileName := fmt.Sprintf("%s.tar", backupDirName)
	tarPath := filepath.ToSlash(filepath.Join(parentDir, tarFileName))

	tui.Logger.Debug("Attempting server-side backup using tar", "tarPath", tarPath)

	if session, err := fs.sshClient.NewSession(); err == nil {
		cmd := fmt.Sprintf("tar -cf '%s' -C '%s' '%s' >/dev/null 2>&1", tarPath, parentDir, baseDirName)
		err = runWithTimeout(session, cmd, 30*time.Second)
		session.Close()
		if err == nil {
			// Successfully created tar.gz backup on the server, return its path
			return tarPath, nil
		}
		// Failed  - fall back to directory copy method
	}

	zipFileName := fmt.Sprintf("%s.zip", backupDirName)
	zipPath := filepath.ToSlash(filepath.Join(parentDir, zipFileName))

	tui.Logger.Debug("Creating fallback zip file...", "path", zipPath)
	zipFile, err := fs.sftpClient.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	mods, err := fs.ListMods()
	if err != nil {
		return "", fmt.Errorf("failed to list mods for backup: %w", err)
	}

	// We need to stream data through local temp files since SFTP does not support server-side copying
	for _, mod := range mods {
		srcPath := filepath.ToSlash(filepath.Join(fs.BaseDir, mod))

		srcFile, err := fs.sftpClient.Open(srcPath)
		if err != nil {
			return "", fmt.Errorf("failed to open source mod for backup: %w", err)
		}

		writer, err := zipWriter.Create(mod)
		if err != nil {
			srcFile.Close()
			return "", fmt.Errorf("failed to create destination mod for backup: %w", err)
		}

		buf, err := io.ReadAll(srcFile)
		srcFile.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read source mod for backup: %w", err)
		}

		if _, err := writer.Write(buf); err != nil {
			return "", fmt.Errorf("failed to write mod to zip: %w", err)
		}
	}
	tui.Logger.Debug("Zip backup successful!")
	return zipPath, nil
}

// Close the SFTP and SSH connections when done
func (fs *SFTPFS) Close() error {
	fs.sftpClient.Close()
	return fs.sshClient.Close()
}

func shellProbe(sshClient *ssh.Client) bool {
	session, err := sshClient.NewSession()
	if err != nil {
		return false
	}

	err = runWithTimeout(session, "echo gorinth-probe", 2*time.Second)
	session.Close()

	return err == nil
}

func runWithTimeout(session *ssh.Session, command string, timeout time.Duration) error {
	// Create a channel to listen for command completion
	errCh := make(chan error, 1)

	// Run the command in a separate goroutine
	go func() {
		errCh <- session.Run(command)
	}()

	// Run against a timer
	select {
	case <-time.After(timeout):
		// Timeout occurred, attempt to kill the session
		_ = session.Signal(ssh.SIGKILL) // Best effort to kill the session
		session.Close()
		return fmt.Errorf("command timed out and session was closed")
	case err := <-errCh:
		// Command completed before timeout
		return err
	}
}

package vfs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPFS struct {
	BaseDir string
	sshClient *ssh.Client
	sftpClient *sftp.Client	
}

// Create a new SSH connection and init SFTP client
func NewSFTPFS(host string, port int, user, pass, dir string) (*SFTPFS, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 10 * time.Second,
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

	return &SFTPFS{
		BaseDir: dir,
		sshClient: sshClient,
		sftpClient: sftpClient,
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
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jar"){
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

func (fs *SFTPFS) Backup() (string, error) {
	timestamp := time.Now().Format("2006-01-02_150405")
	backupDirName := fmt.Sprintf("mods_backup_%s", timestamp)

	parentDir := filepath.Dir(fs.BaseDir)
	backupPath := filepath.ToSlash(filepath.Join(parentDir, backupDirName))

	if err := fs.sftpClient.Mkdir(backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	mods, err := fs.ListMods()
	if err != nil {
		return "", fmt.Errorf("failed to list mods for backup: %w", err)
	}

	// We need to stream data through local temp files since SFTP does not support server-side copying
	for _, mod := range mods {
		srcPath := filepath.ToSlash(filepath.Join(fs.BaseDir, mod))
		dstPath := filepath.ToSlash(filepath.Join(backupPath, mod))

		srcFile, err := fs.sftpClient.Open(srcPath)
		if err != nil {
			return "", fmt.Errorf("failed to open source mod for backup: %w", err)
		}

		dstFile, err := fs.sftpClient.Create(dstPath)
		if err != nil {
			srcFile.Close()
			return "", fmt.Errorf("failed to create destination mod for backup: %w", err)
		}

		io.Copy(dstFile, srcFile)

		srcFile.Close()
		dstFile.Close()
	}

	return backupPath, nil
}

// Close the SFTP and SSH connections when done
func (fs *SFTPFS) Close() error {
	fs.sftpClient.Close()
	return fs.sshClient.Close()
}
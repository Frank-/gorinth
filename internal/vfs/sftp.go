package vfs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPFS struct {
	sftpBase
}

// Create a new SFTPFS instance
func NewSFTPFS(sshClient *ssh.Client, sftpClient *sftp.Client, dir string) (*SFTPFS, error) {

	return &SFTPFS{
		sftpBase: sftpBase{
			BaseDir:    dir,
			sftpClient: sftpClient,
			sshClient:  sshClient,
		},
	}, nil
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

func (fs *SFTPFS) DownloadMod(url string, targetFilename string) error {
	destPath := filepath.ToSlash(filepath.Join(fs.BaseDir, targetFilename))

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
	// timestamp := time.Now().Format("2006-01-02_150405")

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

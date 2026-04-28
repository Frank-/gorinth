package vfs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

type SFTPFS struct {
	sftpBase
	localCacheDir string
	appFS         afero.Fs
}

// Create a new SFTPFS instance
func NewSFTPFS(sshClient *ssh.Client, sftpClient *sftp.Client, dir string, afs afero.Fs) (*SFTPFS, error) {

	return &SFTPFS{
		sftpBase: sftpBase{
			BaseDir:    dir,
			sftpClient: sftpClient,
			sshClient:  sshClient,
		},
		appFS: afs,
	}, nil
}

func (fs *SFTPFS) ensureCache() error {
	if fs.localCacheDir != "" {
		return nil
	}

	tempDir, err := afero.TempDir(fs.appFS, "", "gorinth-sftp-cache-*")
	if err != nil {
		return err
	}

	fs.localCacheDir = tempDir
	return fs.SyncToDir(fs.localCacheDir)
}

// // Stream the file from SFTP and compute its SHA1 hash without loading the entire file into memory
// // TODO: Change to hash remotemod as a standalone command
func (fs *SFTPFS) HashRemoteMod(filename string) (string, error) {
	path := filepath.ToSlash(filepath.Join(fs.BaseDir, filename))
	file, err := fs.sftpClient.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	return computeHash(file)
}

func (fs *SFTPFS) HashMods() (map[string]string, error) {
	// Ensure we have a local cache of the mods to avoid multiple round-trips to the server when hashing multiple files
	if err := fs.ensureCache(); err != nil {
		return nil, err
	}

	return hashLocalDirectory(fs.appFS, fs.localCacheDir)
}

// Download a mod from a URL and save it directly to the SFTP server without storing it locally first
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

// Sync the mods from the SFTP server to a local directory
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
	// Ensure we have a local cache of the mods. This should already exist if hashmods has already run
	if err := fs.ensureCache(); err != nil {
		return "", fmt.Errorf("failed to ensure local cache for backup: %w", err)
	}
	tui.Logger.Info("Creating backup of mods directory...")
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	zipFileName := fmt.Sprintf("%s_backup_%s.zip", baseDirName, timestamp)

	// Create backup directory
	backupDir := "backups"
	fs.appFS.MkdirAll(backupDir, os.ModePerm)

	destZipPath := filepath.Join(backupDir, zipFileName)

	err := zipLocalDirectory(fs.appFS, fs.localCacheDir, destZipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup zip: %w", err)
	}

	// return absolute path to the created zip file
	return filepath.Abs(destZipPath)
}

// Clean up any temporary files created for caching or backup when we close the SFTPFS instance
func (fs *SFTPFS) Close() error {
	if fs.localCacheDir != "" {
		fs.appFS.RemoveAll(fs.localCacheDir)
	}
	return fs.sftpBase.Close()
}

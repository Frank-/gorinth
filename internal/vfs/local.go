package vfs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/spf13/afero"
)

type LocalFS struct {
	BaseDir string
	appFS   afero.Fs
}

func NewLocalFS(dir string, afs afero.Fs) (*LocalFS, error) {
	// info, err := fs.Stat(dir)
	exists, err := afero.Exists(afs, dir)
	if err != nil {
		return nil, fmt.Errorf("error checking directory: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("mods directory does not exist: %s", dir)
	}

	info, err := afs.Stat(dir)
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", dir)
	}
	return &LocalFS{
		BaseDir: dir,
		appFS:   afs,
	}, nil
}

func (fs *LocalFS) ListMods() ([]string, error) {
	var mods []string
	entries, err := afero.ReadDir(fs.appFS, fs.BaseDir)
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

func (fs *LocalFS) HashMods() (map[string]string, error) {
	return hashLocalDirectory(fs.appFS, fs.BaseDir)
}

func (fs *LocalFS) DeleteMod(filename string) error {
	path := filepath.Join(fs.BaseDir, filename)
	return fs.appFS.Remove(path)
}

func (fs *LocalFS) WriteMod(filename string, data io.Reader) error {
	path := filepath.Join(fs.BaseDir, filename)

	// Create new file
	file, err := fs.appFS.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Stream data to file
	_, err = io.Copy(file, data)
	return err
}

func (fs *LocalFS) RenameMod(oldName, newName string) error {
	oldPath := filepath.Join(fs.BaseDir, oldName)
	newPath := filepath.Join(fs.BaseDir, newName)
	return fs.appFS.Rename(oldPath, newPath)
}

func (fs *LocalFS) DownloadMod(url string, targetFilename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download mod: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download mod: received status code %d", resp.StatusCode)
	}

	destPath := filepath.Join(fs.BaseDir, targetFilename)
	file, err := fs.appFS.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create mod file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func (fs *LocalFS) CleanupTmpFiles() error {
	files, err := afero.ReadDir(fs.appFS, fs.BaseDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".gorinth-tmp") {
			path := filepath.Join(fs.BaseDir, f.Name())
			fs.appFS.Remove(path)
		}
	}
	return nil
}

func (fs *LocalFS) Backup(baseDirName string) (string, error) {
	tui.Logger.Info("Creating backup of local mods directory...")

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	zipFileName := fmt.Sprintf("%s_backup_%s.zip", baseDirName, timestamp)

	// Create backup directory
	backupDir := "backups"
	fs.appFS.MkdirAll(backupDir, os.ModePerm)
	destZipPath := filepath.Join(backupDir, zipFileName)

	err := zipLocalDirectory(fs.appFS, fs.BaseDir, destZipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup zip: %w", err)
	}

	// Return absolute path to the created zip file
	return filepath.Abs(destZipPath)
}

func (fs *LocalFS) Close() error {
	// No resources to release for local file system
	return nil
}

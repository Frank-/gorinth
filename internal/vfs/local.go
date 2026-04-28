package vfs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/spf13/afero"
)

type LocalFS struct {
	BaseDir string
	AppFS   afero.Fs
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
		AppFS:   afs,
	}, nil
}

func (fs *LocalFS) ListMods() ([]string, error) {
	var mods []string
	entries, err := afero.ReadDir(fs.AppFS, fs.BaseDir)
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

func (fs *LocalFS) HashMod(filename string) (string, error) {
	path := filepath.Join(fs.BaseDir, filename)
	file, err := fs.AppFS.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Modrinth expects the hash to be in hex format
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (fs *LocalFS) HashMods() (map[string]string, error) {
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

func (fs *LocalFS) DeleteMod(filename string) error {
	path := filepath.Join(fs.BaseDir, filename)
	return fs.AppFS.Remove(path)
}

func (fs *LocalFS) WriteMod(filename string, data io.Reader) error {
	path := filepath.Join(fs.BaseDir, filename)

	// Create new file
	file, err := fs.AppFS.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Stream data to file
	_, err = io.Copy(file, data)
	return err
}

func (fs *LocalFS) Rename(oldName, newName string) error {
	oldPath := filepath.Join(fs.BaseDir, oldName)
	newPath := filepath.Join(fs.BaseDir, newName)
	return fs.AppFS.Rename(oldPath, newPath)
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
	file, err := fs.AppFS.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create mod file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func (fs *LocalFS) SyncToDir(dest string) error {
	mods, err := fs.ListMods()
	if err != nil {
		return fmt.Errorf("failed to list mods for syncing: %w", err)
	}

	for _, mod := range mods {
		srcPath := filepath.Join(fs.BaseDir, mod)
		destPath := filepath.Join(dest, mod)

		srcFile, err := fs.AppFS.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open source mod file '%s': %w", srcPath, err)
		}

		destFile, err := fs.AppFS.Create(destPath)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to create destination mod file '%s': %w", destPath, err)
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			return fmt.Errorf("failed to copy mod file '%s' to '%s': %w", srcPath, destPath, err)
		}
	}

	return nil
}

func (fs *LocalFS) Backup(baseDirName string) (string, error) {
	tui.Logger.Info("Creating backup of local mods directory...")
	mods, err := fs.ListMods()
	if err != nil {
		return "", fmt.Errorf("failed to read mods directory: %w", err)
	}

	// baseDirName := filepath.Base(fs.BaseDir)
	tui.Logger.Debug("Backing up mods", "modCount", len(mods), "baseDir", baseDirName)
	return createLocalZip(baseDirName, mods, func(mod string) (io.ReadCloser, error) {
		return fs.AppFS.Open(filepath.Join(fs.BaseDir, mod))
	})
}

func (fs *LocalFS) Close() error {
	// No resources to release for local file system
	return nil
}

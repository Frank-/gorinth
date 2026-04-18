package vfs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type LocalFS struct {
	BaseDir string
}

func NewLocalFS(dir string) (*LocalFS, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("mod directory does not exist: %s", dir)
	}
	if err != nil {
		return nil, fmt.Errorf("error accessing mod directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("mod path is not a directory: %s", dir)
	}
	return &LocalFS{BaseDir: dir}, nil
}

func (fs *LocalFS) ListMods() ([]string, error) {
	var mods []string
	entries, err := os.ReadDir(fs.BaseDir)
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
	file, err := os.Open(path)
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
	return os.Remove(path)
}

func (fs *LocalFS) WriteMod(filename string, data io.Reader) error {
	path := filepath.Join(fs.BaseDir, filename)

	// Create new file
	file, err := os.Create(path)
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
	return os.Rename(oldPath, newPath)
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
	file, err := os.Create(destPath)
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

		srcFile, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open source mod file '%s': %w", srcPath, err)
		}

		destFile, err := os.Create(destPath)
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

func (fs *LocalFS) Backup() (string, error) {
	mods, err := fs.ListMods()
	if err != nil {
		return "", fmt.Errorf("failed to read mods directory: %w", err)
	}

	baseDirName := filepath.Base(fs.BaseDir)
	return createLocalZip(baseDirName, mods, func(mod string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(fs.BaseDir, mod))
	})
}

func (fs *LocalFS) Close() error {
	// No resources to release for local file system
	return nil
}

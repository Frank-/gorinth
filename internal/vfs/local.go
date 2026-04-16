package vfs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
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
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jar"){
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
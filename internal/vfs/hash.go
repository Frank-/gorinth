package vfs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// Internal helper function to compute hash from an io.Reader
func computeHash(data io.Reader) (string, error) {
	hash := sha1.New()
	if _, err := io.Copy(hash, data); err != nil {
		return "", err
	}

	// Modrinth expects the hash to be in hex format
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Take any afero directory and compute the hash of all .jar files within it, returning a map of filename to hash
func hashLocalDirectory(appFs afero.Fs, dir string) (map[string]string, error) {
	hashes := make(map[string]string)

	files, err := afero.ReadDir(appFs, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read local cache directory: %w", err)
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jar") {
			continue
		}

		path := filepath.Join(dir, f.Name())
		file, err := appFs.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open mod file from cache: %w", err)
		}

		hash, err := computeHash(file)
		file.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to compute hash for mod '%s': %w", f.Name(), err)
		}
		hashes[f.Name()] = hash
	}

	return hashes, nil
}

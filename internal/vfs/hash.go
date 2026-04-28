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

func listJarFiles(appFs afero.Fs, dir string) ([]string, error) {
	var jarFiles []string

	files, err := afero.ReadDir(appFs, dir)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jar") {
			continue
		}
		jarFiles = append(jarFiles, f.Name())
	}

	return jarFiles, nil
}

// Take any afero directory and compute the hash of all .jar files within it, returning a map of filename to hash
func hashLocalDirectory(appFs afero.Fs, dir string) (map[string]string, error) {
	hashes := make(map[string]string)

	filenames, err := listJarFiles(appFs, dir)
	if err != nil {
		return nil, err
	}

	for _, name := range filenames {
		path := filepath.Join(dir, name)
		file, err := appFs.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open mod file from cache: %w", err)
		}

		hash, err := computeHash(file)
		file.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to compute hash for mod '%s': %w", name, err)
		}
		hashes[name] = hash
	}

	return hashes, nil
}

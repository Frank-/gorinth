package vfs

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// Shared base struct for SSH & SFTP implementations as we use SFTP for filesystem operations.

type sftpBase struct {
	sftpClient *sftp.Client
	BaseDir    string
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

func (fs *SFTPFS) DeleteMod(filename string) error {
	path := filepath.ToSlash(filepath.Join(fs.BaseDir, filename))
	return fs.sftpClient.Remove(path)
}

func (fs *SFTPFS) Rename(oldName, newName string) error {
	oldPath := filepath.ToSlash(filepath.Join(fs.BaseDir, oldName))
	newPath := filepath.ToSlash(filepath.Join(fs.BaseDir, newName))
	return fs.sftpClient.Rename(oldPath, newPath)
}

// Close the SFTP and SSH connections when done
func (fs *SFTPFS) Close() error {
	fs.sftpClient.Close()
	return fs.sshClient.Close()
}

package vfs

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/Frank-/gorinth/internal/tui"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Shared base struct for SSH & SFTP implementations as we use SFTP for filesystem operations.

type sftpBase struct {
	sftpClient *sftp.Client
	sshClient  *ssh.Client
	BaseDir    string
}

func (base *sftpBase) ListMods() ([]string, error) {
	var mods []string
	tui.Logger.Debug("Listing mods in remote directory", "directory", base.BaseDir)
	entries, err := base.sftpClient.ReadDir(base.BaseDir)
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

func (base *sftpBase) WriteMod(filename string, data io.Reader) error {
	path := filepath.ToSlash(filepath.Join(base.BaseDir, filename))
	file, err := base.sftpClient.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	return err
}

func (base *sftpBase) DeleteMod(filename string) error {
	path := filepath.ToSlash(filepath.Join(base.BaseDir, filename))
	return base.sftpClient.Remove(path)
}

func (base *sftpBase) RenameMod(oldName, newName string) error {
	oldPath := filepath.ToSlash(filepath.Join(base.BaseDir, oldName))
	newPath := filepath.ToSlash(filepath.Join(base.BaseDir, newName))
	return base.sftpClient.Rename(oldPath, newPath)
}

func (base *sftpBase) CleanupTmpFiles() error {
	files, err := base.sftpClient.ReadDir(base.BaseDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), TmpFileSuffix) {
			base.DeleteMod(file.Name())
		}
	}
	return nil
}

// Close the SFTP and SSH connections when done
func (base *sftpBase) Close() error {
	// Clean up temp files too
	base.CleanupTmpFiles()

	base.sftpClient.Close()
	return base.sshClient.Close()
}

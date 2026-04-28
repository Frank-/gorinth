package vfs

import (
	"io"
)

type FileSystem interface {
	ListMods() ([]string, error)
	// HashMod(filename string) (string, error)
	HashMods() (map[string]string, error)
	WriteMod(filename string, data io.Reader) error
	DownloadMod(url string, targetFilename string) error
	DeleteMod(filename string) error
	RenameMod(oldName, newName string) error
	CleanupTmpFiles() error
	Backup(baseDirName string) (string, error)
	Close() error
}

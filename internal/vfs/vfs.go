package vfs

import "io"

type FileSystem interface {
	ListMods() ([]string, error)
	HashMod(filename string) (string, error)
	HashMods() (map[string]string, error)
	DeleteMod(filename string) error
	WriteMod(filename string, data io.Reader) error
	Rename(oldName, newName string) error
	DownloadMod(url string, targetFilename string) error
	Backup(baseDirName string) (string, error)
	Close() error
}

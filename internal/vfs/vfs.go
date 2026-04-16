package vfs

import "io"


type FileSystem interface {
	// ListMods returns a list of mod file names in the mods directory
	ListMods() ([]string, error)

	// HashMod calculates the SHA-1 hash of a mod file given its filename
	HashMod(filename string) (string, error)

	// DeleteMod removes the old mod file
	DeleteMod(filename string) error

	// WriteMod writes a new mod file to the mods directory
	WriteMod(filename string, data io.Reader) error
}
package vfs_test

import (
	"path/filepath"
	"testing"

	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/spf13/afero"
)

func TestLocalFS_ListMods(t *testing.T) {
	memFS := afero.NewMemMapFs()
	baseDir := "/mods"

	// Set up virtual file system with some test files
	memFS.MkdirAll(baseDir, 0755)
	afero.WriteFile(memFS, filepath.Join(baseDir, "mod1.jar"), []byte("mod1 content"), 0644)
	afero.WriteFile(memFS, filepath.Join(baseDir, "mod2.jar"), []byte("mod2 content"), 0644)
	afero.WriteFile(memFS, filepath.Join(baseDir, "not_a_mod.txt"), []byte("not a mod content"), 0644)

	l, err := vfs.NewLocalFS(baseDir, memFS)
	if err != nil {
		t.Fatalf("Failed to init: %v", err)
	}

	mods, err := l.ListMods()

	if err != nil {
		t.Fatalf("ListMods failed: %v", err)
	}

	if len(mods) != 2 {
		t.Fatalf("Expected 2 mods, got %d", len(mods))
	}

	expected := map[string]bool{"mod1.jar": true, "mod2.jar": true}
	for _, mod := range mods {
		if !expected[mod] {
			t.Errorf("Unexpected mod found: %s", mod)
		}
	}
}

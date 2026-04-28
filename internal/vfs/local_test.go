package vfs_test

import (
	"path/filepath"
	"strings"
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

func TestLocalFS_Operations(t *testing.T) {
	memFS := afero.NewMemMapFs()
	baseDir := "/mods"
	memFS.MkdirAll(baseDir, 0755)

	fs, err := vfs.NewLocalFS(baseDir, memFS)
	if err != nil {
		t.Fatalf("Failed to init: %v", err)
	}

	// Test WriteMod
	modName := "testmod.jar"
	content := "test content"
	err = fs.WriteMod(modName, strings.NewReader(content))
	if err != nil {
		t.Fatalf("WriteMod failed: %v", err)
	}

	// Verify file exists
	data, _ := afero.ReadFile(memFS, filepath.Join(baseDir, modName))
	if string(data) != content {
		t.Errorf("Expected content %s, got %s", content, string(data))
	}

	// Test RenameMod
	newName := "renamed.jar"
	err = fs.RenameMod(modName, newName)
	if err != nil {
		t.Fatalf("RenameMod failed: %v", err)
	}

	exists, _ := afero.Exists(memFS, filepath.Join(baseDir, newName))
	if !exists {
		t.Error("Renamed file does not exist")
	}

	// Test DeleteMod
	err = fs.DeleteMod(newName)
	if err != nil {
		t.Fatalf("DeleteMod failed: %v", err)
	}

	exists, _ = afero.Exists(memFS, filepath.Join(baseDir, newName))
	if exists {
		t.Error("Deleted file still exists")
	}
}

func TestLocalFS_CleanupTmpFiles(t *testing.T) {
	memFS := afero.NewMemMapFs()
	baseDir := "/mods"
	memFS.MkdirAll(baseDir, 0755)

	// Create some temp files
	afero.WriteFile(memFS, filepath.Join(baseDir, "mod1.jar"+vfs.TmpFileSuffix), []byte("tmp"), 0644)
	afero.WriteFile(memFS, filepath.Join(baseDir, "normal.jar"), []byte("normal"), 0644)

	fs, _ := vfs.NewLocalFS(baseDir, memFS)
	err := fs.CleanupTmpFiles()
	if err != nil {
		t.Fatalf("CleanupTmpFiles failed: %v", err)
	}

	// Verify tmp file is gone
	exists, _ := afero.Exists(memFS, filepath.Join(baseDir, "mod1.jar"+vfs.TmpFileSuffix))
	if exists {
		t.Error("Tmp file still exists after cleanup")
	}

	// Verify normal file remains
	exists, _ = afero.Exists(memFS, filepath.Join(baseDir, "normal.jar"))
	if !exists {
		t.Error("Normal file was accidentally deleted")
	}
}

package vfs

import (
	"archive/zip"
	"io"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestZipLocalDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()
	sourceDir := "/mods"
	destZip := "/backup.zip"
	fs.MkdirAll(sourceDir, 0755)

	// Create test mods
	afero.WriteFile(fs, sourceDir+"/mod1.jar", []byte("mod1 content"), 0644)
	afero.WriteFile(fs, sourceDir+"/mod2.jar", []byte("mod2 content"), 0644)
	afero.WriteFile(fs, sourceDir+"/readme.txt", []byte("ignore me"), 0644)

	err := zipLocalDirectory(fs, sourceDir, destZip)
	if err != nil {
		t.Fatalf("zipLocalDirectory failed: %v", err)
	}

	// Verify zip exists
	exists, _ := afero.Exists(fs, destZip)
	if !exists {
		t.Fatal("Zip file was not created")
	}

	// Open and verify zip content
	f, _ := fs.Open(destZip)
	defer f.Close()

	stat, _ := f.Stat()
	zipReader, err := zip.NewReader(f, stat.Size())
	if err != nil {
		t.Fatalf("Failed to open zip reader: %v", err)
	}

	foundMods := 0
	for _, zipFile := range zipReader.File {
		if zipFile.Name == "mod1.jar" || zipFile.Name == "mod2.jar" {
			foundMods++
			rc, _ := zipFile.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(content), "content") {
				t.Errorf("Unexpected content in %s", zipFile.Name)
			}
		}
		if zipFile.Name == "readme.txt" {
			t.Error("readme.txt should not have been included in the zip")
		}
	}

	if foundMods != 2 {
		t.Errorf("Expected 2 mods in zip, found %d", foundMods)
	}
}

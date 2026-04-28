package vfs

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestComputeHash(t *testing.T) {
	content := "hello world"
	// SHA1 of "hello world" is 2aae6c35c94fcfb415dbe95f408b9ce91ee846ed
	expected := "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed"

	reader := strings.NewReader(content)
	hash, err := computeHash(reader)
	if err != nil {
		t.Fatalf("computeHash failed: %v", err)
	}

	if hash != expected {
		t.Errorf("expected hash %s, got %s", expected, hash)
	}
}

func TestListJarFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	dir := "/testmods"
	fs.MkdirAll(dir, 0755)

	testFiles := []string{
		"mod1.jar",
		"mod2.jar",
		"readme.txt",
		"subfolder/mod3.jar",
	}

	for _, f := range testFiles {
		afero.WriteFile(fs, dir+"/"+f, []byte("content"), 0644)
	}

	jars, err := listJarFiles(fs, dir)
	if err != nil {
		t.Fatalf("listJarFiles failed: %v", err)
	}

	if len(jars) != 2 {
		t.Errorf("expected 2 jars, got %d", len(jars))
	}

	for _, jar := range jars {
		if !strings.HasSuffix(jar, ".jar") {
			t.Errorf("non-jar file found: %s", jar)
		}
		if strings.Contains(jar, "/") {
			t.Errorf("filename should not contain path: %s", jar)
		}
	}
}

func TestHashLocalDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()
	dir := "/testmods"
	fs.MkdirAll(dir, 0755)

	// "mod1" -> sha1: fc33504b2cf8a15b824f8477e417986b2315c869
	afero.WriteFile(fs, dir+"/mod1.jar", []byte("mod1"), 0644)
	
	hashes, err := hashLocalDirectory(fs, dir)
	if err != nil {
		t.Fatalf("hashLocalDirectory failed: %v", err)
	}

	if len(hashes) != 1 {
		t.Errorf("expected 1 hash, got %d", len(hashes))
	}

	expectedHash := "fc33504b2cf8a15b824f8477e417986b2315c869"
	if hashes["mod1.jar"] != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, hashes["mod1.jar"])
	}
}

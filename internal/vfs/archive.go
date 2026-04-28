package vfs

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

func zipLocalDirectory(appFS afero.Fs, sourceDir string, destZipPath string) error {
	zipFile, err := appFS.Create(destZipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	files, err := afero.ReadDir(appFS, sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".jar") {
			continue
		}

		srcPath := filepath.Join(sourceDir, file.Name())
		srcFile, err := appFS.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}

		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:   file.Name(),
			Method: zip.Store,
		})
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		_, err = io.Copy(writer, srcFile)
		srcFile.Close()
		if err != nil {
			return fmt.Errorf("failed to write file to zip: %w", err)
		}
	}

	return nil
}

package vfs

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func createLocalZip(baseDirName string, mods []string, openFile func(mod string) (io.ReadCloser, error)) (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// Ensure we have a local directory to store the backup in
	cwd, _ := os.Getwd()
	localBackupDir := filepath.Join(cwd, "backups")
	if err := os.MkdirAll(localBackupDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create local backup directory: %w", err)
	}

	zipFileName := fmt.Sprintf("%s_backup_%s.zip", baseDirName, timestamp)
	localZipPath := filepath.Join(localBackupDir, zipFileName)

	// Create the zip file
	zipFile, err := os.Create(localZipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, mod := range mods {
		srcFile, err := openFile(mod)
		if err != nil {
			continue // Skip files that can't be opened
		}

		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:   mod,
			Method: zip.Store,
		})

		if err == nil {
			io.Copy(writer, srcFile)
		}

		srcFile.Close()
	}

	absPath, _ := filepath.Abs(localZipPath)
	return absPath, nil
}

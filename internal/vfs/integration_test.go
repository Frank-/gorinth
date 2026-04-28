package vfs_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/ssh"
)

// setupTestContainer spins up a real SSH/SFTP server for testing.
// We use linuxserver/openssh-server because it provides a full shell for testing remote commands.
func setupTestContainer(ctx context.Context, t *testing.T) (string, string, func()) {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "linuxserver/openssh-server:latest",
		ExposedPorts: []string{"2222/tcp"},
		Env: map[string]string{
			"USER_PASSWORD":  "pass",
			"USER_NAME":      "user",
			"PASSWORD_ACCESS": "true",
		},
		WaitingFor: wait.ForLog("sshd is listening on port 2222"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "2222")
	
	return host, port.Port(), func() { container.Terminate(ctx) }
}

func TestSFTP_FullIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	host, port, cleanup := setupTestContainer(ctx, t)
	defer cleanup()

	// 1. Setup Connection (with retries for startup stability)
	config := &ssh.ClientConfig{
		User: "user",
		Auth: []ssh.AuthMethod{ssh.Password("pass")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	
	addr := fmt.Sprintf("%s:%s", host, port)
	
	var sshClient *ssh.Client
	var err error
	for i := 0; i < 10; i++ {
		sshClient, err = ssh.Dial("tcp", addr, config)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("SSH Dial failed after retries: %v", err)
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		t.Fatalf("SFTP Client init failed: %v", err)
	}
	defer sftpClient.Close()

	// 2. Prepare Remote Files
	// linuxserver/openssh-server user home is usually /config
	remoteDir := "/config/mods" 
	sftpClient.MkdirAll(remoteDir)

	testFile := "test.jar"
	content := "test"
	// SHA1 of "test": a94a8fe5ccb19ba61c4c0873d391e987982fbbd3
	expectedHash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"

	f, err := sftpClient.Create(remoteDir + "/" + testFile)
	if err != nil {
		t.Fatalf("Failed to create remote test file: %v", err)
	}
	f.Write([]byte(content))
	f.Close()

	// 3. Test SFTPFS (Cached Hashing)
	t.Run("SFTPFS_CachedHashing", func(t *testing.T) {
		memFS := afero.NewMemMapFs()
		fs, err := vfs.NewSFTPFS(sshClient, sftpClient, remoteDir, memFS)
		if err != nil {
			t.Fatalf("NewSFTPFS failed: %v", err)
		}
		
		hashes, err := fs.HashMods()
		if err != nil {
			t.Fatalf("HashMods failed: %v", err)
		}

		if hashes[testFile] != expectedHash {
			t.Errorf("Expected hash %s, got %s", expectedHash, hashes[testFile])
		}
	})

	// 4. Test SSHFS (Remote Zero-Bandwidth Hashing)
	t.Run("SSHFS_RemoteHashing", func(t *testing.T) {
		fs, err := vfs.NewSSHFS(sshClient, sftpClient, remoteDir)
		if err != nil {
			t.Fatalf("NewSSHFS failed: %v", err)
		}
		
		hash, err := fs.HashMod(testFile)
		if err != nil {
			t.Fatalf("Remote HashMod failed: %v", err)
		}

		if hash != expectedHash {
			t.Errorf("Expected remote hash %s, got %s", expectedHash, hash)
		}
	})

	// 5. Test File Operations (Deduplicated via sftpBase)
	t.Run("SFTP_Operations", func(t *testing.T) {
		fs, _ := vfs.NewSFTPFS(sshClient, sftpClient, remoteDir, afero.NewMemMapFs())
		
		// Write
		err := fs.WriteMod("new.jar", strings.NewReader("new content"))
		if err != nil {
			t.Errorf("WriteMod failed: %v", err)
		}

		// List
		mods, _ := fs.ListMods()
		found := false
		for _, m := range mods {
			if m == "new.jar" {
				found = true
				break
			}
		}
		if !found {
			t.Error("new.jar not found in ListMods")
		}

		// Rename
		err = fs.RenameMod("new.jar", "renamed.jar")
		if err != nil {
			t.Errorf("RenameMod failed: %v", err)
		}

		// Delete
		err = fs.DeleteMod("renamed.jar")
		if err != nil {
			t.Errorf("DeleteMod failed: %v", err)
		}
	})
}

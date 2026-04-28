package main

import (
	"fmt"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

func connectAndMount(baseFS afero.Fs) (vfs.FileSystem, error) {
	if AppConfig.Mode == "local" {
		tui.Logger.Info("Operating in local mode, using local filesystem")
		return vfs.NewLocalFS(AppConfig.ModsDir, baseFS)
	}

	config := &ssh.ClientConfig{
		User: AppConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(AppConfig.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%d", AppConfig.Host, AppConfig.Port)
	sshClient, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	if AppConfig.Mode == "ssh" {
		return vfs.NewSSHFS(sshClient, sftpClient, AppConfig.ModsDir)
	}

	return vfs.NewSFTPFS(sshClient, sftpClient, AppConfig.ModsDir)
}

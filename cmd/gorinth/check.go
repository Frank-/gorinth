package main

import (
	"fmt"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/spf13/cobra"
)

	var checkCmd = &cobra.Command{
		Use:   "check",
		Short: "Check for updates to your Minecraft server mods without applying them.",
		Long:  `The check command allows you to see if there are any updates available for your Minecraft server mods without actually downloading or applying them. This is useful for planning updates and ensuring compatibility before making changes to your server.`,
		Run: func(cmd *cobra.Command, args []string) {
			tui.Logger.Infof("Starting Gorinth in %s mode", AppConfig.Mode)

			var fs vfs.FileSystem
			var err error


			// Initialize the appropriate file system based on the mode
			spinner, _ := tui.StartSpinner(fmt.Sprintf("Connecting to %s...", AppConfig.Mode))

			if AppConfig.Mode == "local" {
				fs, err = vfs.NewLocalFS(AppConfig.Dir)
			} else if AppConfig.Mode == "sftp" {
				fs, err = vfs.NewSFTPFS(AppConfig.SFTPHost, AppConfig.SFTPPort, AppConfig.SFTPUser, AppConfig.SFTPPassword, AppConfig.Dir)
			}

			if err != nil {
				spinner.Fail("Failed to connect")
				tui.Logger.Fatal("Initialization error", "error", err)
			}

			// Success
			spinner.Success("Connected successfully!")

			defer fs.Close()

			// Start a spinner while we hash the mods
			spinner, _ = tui.StartSpinner("Scanning mods directory..")
			mods, err := fs.ListMods()
			if err != nil {
				spinner.Fail("Failed to list mods")
				tui.Logger.Fatal("Error listing mods", "error", err)
			}
			spinner.Success(fmt.Sprintf("Found %d .jar files", len(mods)))

			spinner, _ = tui.StartSpinner("Computing SHA-1 hashes...")

			// Store hashes in a map for later use
			hashes := make(map[string]string)
			for _, mod := range mods {
				hash, err := fs.HashMod(mod)
				if err != nil {
					spinner.Fail(fmt.Sprintf("Failed to hash mod: %s", mod))
					tui.Logger.Error("Error hashing mod", "mod", mod, "error", err)
					continue
				}
				hashes[mod] = hash
			}

			// Success
			spinner.Success(fmt.Sprintf("Computed hashes for %d mods", len(hashes)))


			// Temporary debug output to verify it worked:
			for mod, hash := range hashes {
				tui.Logger.Debug("Hashed file", "mod", mod, "sha1", hash)
			}

			tui.Logger.Info(fmt.Sprintf("Computed hashes for %d mods", len(hashes)))
			tui.Logger.Info("Ready to query Modrinth API", "mc_version", AppConfig.MCVersion)
			
		},
	}
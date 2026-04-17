package main

import (
	"fmt"
	"sort"

	"context"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/pterm/pterm"
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

		switch AppConfig.Mode {
		case "local":
			fs, err = vfs.NewLocalFS(AppConfig.Dir)
		case "sftp":
			fs, err = vfs.NewSFTPFS(AppConfig.SFTPHost, AppConfig.SFTPPort, AppConfig.SFTPUser, AppConfig.SFTPPassword, AppConfig.Dir)
		default:
			tui.Logger.Fatal("Invalid mode specified", "mode", AppConfig.Mode)
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
		// hashes := make(map[string]string)
		hashList := make([]string, 0, len(mods))
		filenameMap := make(map[string]string)
		filenameMapReversed := make(map[string]string)

		for _, mod := range mods {
			hash, err := fs.HashMod(mod)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Failed to hash mod: %s", mod))
				tui.Logger.Error("Error hashing mod", "mod", mod, "error", err)
				continue
			}
			hashList = append(hashList, hash)
			filenameMap[hash] = mod
			filenameMapReversed[mod] = hash
		}

		// Success
		spinner.Success(fmt.Sprintf("Hashed %d mods", len(hashList)))

		// Debug output to verify it worked:
		for hash, mod := range filenameMap {
			tui.Logger.Debug("Hashed file", "mod", mod, "sha1", hash)
		}

		tui.Logger.Info("Ready to query Modrinth API", "mc_version", AppConfig.GameVersion)

		spinner, _ = tui.StartSpinner("Querying Modrinth API...")
		client := modrinth.NewClient("Frank-", "gorinth", "0.1.0")

		currentVersions, err := client.CheckVersionsFromHashes(context.Background(), hashList)

		if err != nil {
			spinner.Fail("Failed to check current versions")
			tui.Logger.Fatal("Error checking for current versions", "error", err)
		}
		updates, err := client.CheckForUpdates(context.Background(), hashList, AppConfig.GameVersion, AppConfig.Loader)

		if err != nil {
			spinner.Fail("Failed to check for updates")
			tui.Logger.Fatal("Error checking for updates", "error", err)
		}

		spinner, _ = tui.StartSpinner("Fetching mod names...")
		projectIDSet := make(map[string]bool)
		for _, version := range currentVersions {
			projectIDSet[version.ProjectID] = true
		}
		for _, version := range updates {
			projectIDSet[version.ProjectID] = true
		}

		var projectIDs []string
		for id := range projectIDSet {
			projectIDs = append(projectIDs, id)
		}

		projects, err := client.GetProjects(projectIDs)
		if err != nil {
			spinner.Fail("Failed to fetch mod names")
			tui.Logger.Fatal("Error fetching mod names", "error", err)
		}

		spinner.Success("API Check complete")

		tableData := pterm.TableData{
			{"Status", "Mod", "Current Ver", "New Ver", "Compatibility"},
		}

		sort.Strings(mods)

		for _, filename := range mods {
			h := filenameMapReversed[filename]

			current, isKnown := currentVersions[h]
			update, hasUpdate := updates[h]

			displayName := filename

			status := pterm.LightCyan("Up to date")
			currentVer := "-"
			newVer := "-"
			compatibility := pterm.LightGreen("Compatible")

			if isKnown {
				currentVer = current.VersionNumber
				if project, exists := projects[current.ProjectID]; exists {
					displayName = pterm.Bold.Sprint(project.Title)
				}
			}

			// Check if mod supports target mc version
			supportsTarget := false
			if isKnown {
				for _, gv := range current.GameVersions {
					if gv == AppConfig.GameVersion {
						supportsTarget = true
						break
					}

				}

				if !supportsTarget && isKnown {
					status = pterm.LightRed("Incompatible")
					compatibility = pterm.LightRed("Needs update")
				}

				if hasUpdate {
					status = pterm.LightYellow("Update available")
					newVer = pterm.Bold.Sprint(update.VersionNumber)
					compatibility = pterm.LightGreen("Available")
				}

				tableData = append(tableData, []string{status, displayName, currentVer, newVer, compatibility})
			}

		}

		err = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
		if err != nil {
			tui.Logger.Error("Error rendering table", "error", err)
		}

	},
}

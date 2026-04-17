package main

import (
	"fmt"
	"net/http"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type UpdateTask struct {
	DisplayName    string
	OldFilename    string
	NewFilename    string
	CurrentVersion string
	NewVersion     string
	DownloadURL    string
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and apply them to your Minecraft server mods.",
	Long:  `The update command checks for available updates for your Minecraft server mods and applies them automatically. This command will download the latest versions of your mods from Modrinth and replace the old versions in your server's mod directory, ensuring that your server is always running the most up-to-date mods.`,
	Run: func(cmd *cobra.Command, args []string) {
		tui.Logger.Infof("Starting Gorinth Update in %s mode", AppConfig.Mode)
		state, err := FetchGorinthState()
		if err != nil {
			tui.Logger.Fatal("Failed to fetch Gorinth state", "error", err)
		}

		defer state.FS.Close()

		var tasks []UpdateTask

		for _, filename := range state.Mods {
			hash := state.FilenameMapReversed[filename]
			update, hasUpdate := state.Updates[hash]
			current, isKnown := state.CurrentVersions[hash]

			if !hasUpdate || !isKnown {
				continue
			}

			displayName := filename
			if project, exists := state.Projects[update.ProjectID]; exists {
				displayName = project.Title
			}

			var downloadURL, newFilename string
			for _, file := range update.Files {
				if file.Primary {
					downloadURL = file.URL
					newFilename = file.Filename
					break
				}
			}

			// If nopriary file marked just take the first one
			if downloadURL == "" && len(update.Files) > 0 {
				downloadURL = update.Files[0].URL
				newFilename = update.Files[0].Filename
			}

			tasks = append(tasks, UpdateTask{
				OldFilename:    filename,
				NewFilename:    newFilename,
				DownloadURL:    downloadURL,
				DisplayName:    displayName,
				CurrentVersion: current.VersionNumber,
				NewVersion:     update.VersionNumber,
			})
		}

		if len(tasks) == 0 {
			tui.Logger.Info("All mods are up to date! Nothing to do.")
			return
		}

		pterm.Info.Printf("Found %d mods with updates available. Starting update process...\n", len(tasks))

		/*
		* Backup current mods before making any changes. This way if anything goes
		* wrong during the update process, we can restore the original mods from the backup.
		 */
		spinner, _ := tui.StartSpinner("Creating backup...")
		backupPath, err := state.FS.Backup()
		if err != nil {
			spinner.Fail("Failed to create backup")
			tui.Logger.Fatal("Error creating backup", "error", err)
		}
		spinner.Success(fmt.Sprintf("Backup created at %s", backupPath))

		/*
		* Proceed with downloading and applying updates.
		 */
		successCount := 0
		for _, task := range tasks {

			updateText := fmt.Sprintf("Updating %s (%s -> %s)", task.DisplayName, task.CurrentVersion, task.NewVersion)

			spinner, _ = tui.StartSpinner(updateText)

			// Download the new mod file
			resp, err := http.Get(task.DownloadURL)
			if err != nil {
				tui.Logger.Error("Failed to download mod update", "mod", task.NewFilename, "error", err)
				continue
			}

			tmpName := task.NewFilename + ".tmp"
			// Stream directly to server
			if err := state.FS.WriteMod(tmpName, resp.Body); err != nil {
				resp.Body.Close()
				spinner.Fail("Failed to write to disk/server")
				continue
			}
			defer resp.Body.Close()

			if err := state.FS.DeleteMod(task.OldFilename); err != nil {
				tui.Logger.Error("Failed to delete old mod", "mod", task.OldFilename, "error", err)
			}

			if err := state.FS.Rename(tmpName, task.NewFilename); err != nil {
				tui.Logger.Error("Failed to rename new mod", "mod", task.NewFilename, "error", err)
				continue
			}

			successCount++
			spinner.Success(fmt.Sprintf("Successfully updated %s (%s -> %s)", task.DisplayName, task.CurrentVersion, task.NewVersion))
		}

		pterm.Success.Printf("Update process completed! %d/%d mods updated successfully.", successCount, len(tasks))

	},
}

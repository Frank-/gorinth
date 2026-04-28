package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type UpdateTask struct {
	OldFilename    string
	NewFilename    string
	DownloadURL    string
	DisplayName    string
	CurrentVersion string
	NewVersion     string
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and apply them to your Minecraft server mods.",
	Long:  `The update command checks for available updates for your Minecraft server mods and applies them automatically. This command will download the latest versions of your mods from Modrinth and replace the old versions in your server's mod directory, ensuring that your server is always running the most up-to-date mods.`,
	RunE: WithGorinthState(func(cmd *cobra.Command, args []string, state *GorinthState) error {
		tui.Logger.Infof("Starting Gorinth Update in %s mode", AppConfig.Mode)

		var tasks []UpdateTask
		var brokenMods []string

		for _, filename := range state.Mods {
			hash := state.FilenameMapReversed[filename]
			update, hasUpdate := state.Updates[hash]
			current, isKnown := state.CurrentVersions[hash]

			// If the mod is not known to Modrinth warn and skip
			if !isKnown {
				tui.Logger.Warn("Unmanaged mod bypassed (Not on Modrinth)", "mod", filename)
				continue
			}

			if hasUpdate && isKnown && update.ID == current.ID {
				hasUpdate = false
			}

			supportsTarget := false
			if isKnown {
				for _, gv := range current.GameVersions {
					if gv == AppConfig.GameVersion {
						supportsTarget = true
						break
					}
				}
			}

			// If mod does not support target and has no update, it is effectively broken for the target version
			if isKnown && !supportsTarget && !hasUpdate {
				brokenMods = append(brokenMods, filename)
				tui.Logger.Warn("Mod is incompatible with target Minecraft version and has no update available", "mod", filename, "target_version", AppConfig.GameVersion)
			}

			// No updates needed, skip to next mod
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

			// If no primary file marked just take the first one
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

		// If there are broken mods and force flag is not set, abort the update process to prevent potential issues with the server. This is a safety measure to ensure that users are aware of compatibility issues before proceeding with updates.
		if len(brokenMods) > 0 {
			tableData := pterm.TableData{buildTableHeader()}
			sort.Strings(brokenMods)

			for _, filename := range brokenMods {
				hash := state.FilenameMapReversed[filename]
				current, isKnown := state.CurrentVersions[hash]

				if isKnown {
					supportsTarget := false
					for _, gv := range current.GameVersions {
						if gv == AppConfig.GameVersion {
							supportsTarget = true
							break
						}
					}

					if !supportsTarget {
						row := buildTableRow(filename, state.CurrentVersions, state.Updates, state.Projects, hash, AppConfig.GameVersion)
						tableData = append(tableData, row)
					}
				}
			}

			_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

			if !AppConfig.Force {
				pterm.Error.Printfln("Update aborted! %d mods are incompatible with %s and have no updates.", len(brokenMods), AppConfig.GameVersion)
				pterm.Warning.Printfln("Continuing would break your server. Use --force to bypass this check (not recommended).")
				return nil
			} else {
				pterm.Warning.Printfln("DANGER: Force flag detected. Proceeding with update despite %d incompatible mods.", len(brokenMods))
				pterm.Info.Printfln("A backup will be created before any files are modified so you can roll back if the server crashes.")
			}

		}

		if len(tasks) == 0 {
			tui.Logger.Info("All mods are up to date! Nothing to do.")
			return nil
		}

		pterm.Info.Printfln("Found %d mods with updates available. Starting update process...", len(tasks))

		/*
		* Backup current mods before making any changes.
		 */

		if !AppConfig.SkipBackup {
			if err := state.performBackup(); err != nil {
				tui.Logger.Error("Failed to create backup before updates", "error", err)
				pterm.Warning.Printfln("Proceeding without backup! This is not recommended. Use --skip-backup to bypass this warning.")
			} else {
				pterm.Success.Printfln("Backup created successfully! Backup file will be retained after updates.")
			}

			if AppConfig.Force && len(brokenMods) > 0 {
				pterm.Info.Printf("Recovery Tip: If your server fails to start, delete your current mods folder and restore from the backup above.\n\n")
			}
		} else {
			tui.Logger.Warn("Skipping backup creation as per configuration. (--skip-backup enabled)")
		}

		/*
		* Proceed with downloading and applying updates.
		 */

		successCount := 0
		for _, task := range tasks {

			updateText := fmt.Sprintf("Updating %s (%s -> %s)", task.DisplayName, task.CurrentVersion, task.NewVersion)
			spinner, _ := tui.StartSpinner(updateText)

			tmpName := task.NewFilename + ".tmp"
			if err := state.TargetFS.DownloadMod(task.DownloadURL, tmpName); err != nil {
				spinner.Fail("Download failed")
				tui.Logger.Error("Failed to download mod update", "mod", task.NewFilename, "error", err)
				continue
			}

			if err := state.TargetFS.DeleteMod(task.OldFilename); err != nil {
				tui.Logger.Error("Failed to delete old mod", "mod", task.OldFilename, "error", err)
			}

			if err := state.TargetFS.RenameMod(tmpName, task.NewFilename); err != nil {
				spinner.Fail("Failed to finalize file name")
				tui.Logger.Error("Failed to rename new mod", "mod", task.NewFilename, "error", err)
				continue
			}

			successCount++
			spinner.Success(fmt.Sprintf("Successfully updated %s (%s -> %s)", task.DisplayName, task.CurrentVersion, task.NewVersion))
		}

		pterm.Success.Printfln("Update process completed! %d/%d mods updated successfully.", successCount, len(tasks))

		if successCount < len(tasks) {
			failedCount := len(tasks) - successCount
			pterm.Error.Printfln("%d update(s) failed to apply. Check the logs above for details.", failedCount)
		}

		if len(brokenMods) > 0 {
			pterm.Warning.Printfln("%d incompatible mod(s) were left untouched to prevent immediate server crashes.", len(brokenMods))
		}

		return nil
	}),
}

func (state *GorinthState) performBackup() error {
	spinner, _ := tui.StartSpinner("Creating backup...")

	realName := filepath.Base(AppConfig.ModsDir)

	backupPath, err := state.TargetFS.Backup(realName)
	if err != nil {
		spinner.Fail("Failed to create backup")
		return err
	}

	spinner.Success(fmt.Sprintf("Backup created at %s", backupPath))
	return nil
}

package main

import (
	"fmt"
	"sort"
	"strings"

	"context"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// Helper function to truncate long mod names for better table display
func truncateText(text string, maxLength int) string {
	// If no truncation is desired, return the original text
	if AppConfig.NoTruncate {
		return text
	}

	runes := []rune(text)
	if len(runes) > maxLength {
		return string(runes[:maxLength-3]) + "..."
	}
	return text
}

// Helper function to format Minecraft versions for display in the table
func formatMCVersions(versions []string) string {
	if len(versions) == 0 {
		return "-"
	}

	if len(versions) <= 2 {
		return strings.Join(versions, ", ")
	}

	return fmt.Sprintf("%s (+%d)", versions[0], len(versions)-1)
}

func buildTableHeader() []string {
	return []string{"Status", "Mod", "Current Mod Ver", "MC (Current)", "New Mod Ver", "MC (New)", "Compatibility"}
}

func buildTableRow(
	filename string,
	currentVerMap map[string]modrinth.Version,
	updatesMap map[string]modrinth.Version,
	projectsMap map[string]modrinth.ModrinthProject,
	hash string,
	targetVersion string,
) []string {
	current, isKNown := currentVerMap[hash]
	update, hasUpdate := updatesMap[hash]

	displayName := filename
	status := pterm.LightCyan("Up to date")
	currentVer := "-"
	currentMC := "-"
	newVer := "-"
	newMC := "-"
	compatibility := pterm.LightGreen("Compatible")

	if isKNown {
		currentVer = truncateText(current.VersionNumber, 18)
		currentMC = formatMCVersions(current.GameVersions)
		if project, exists := projectsMap[current.ProjectID]; exists {
			rawTitle := truncateText(project.Title, 35)
			displayName = pterm.Bold.Sprint(rawTitle)
		}

		supportsTarget := false
		for _, gv := range current.GameVersions {
			if gv == targetVersion {
				supportsTarget = true
				break
			}

		}

		if !supportsTarget && isKNown {
			status = pterm.LightRed("Incompatible")
			compatibility = pterm.LightRed("Needs update")
		}

		if hasUpdate {
			status = pterm.LightYellow("Update available")
			rawNewVer := truncateText(update.VersionNumber, 18)
			newVer = pterm.Bold.Sprint(rawNewVer)
			newMC = pterm.LightYellow(formatMCVersions(update.GameVersions))
			compatibility = pterm.LightGreen("Available")
		}

	}

	if !isKNown {
		displayName = truncateText(filename, 35)
		status = pterm.LightRed("Unknown")
		compatibility = pterm.LightRed("Unknown")
	}

	return []string{status, displayName, currentVer, currentMC, newVer, newMC, compatibility}
}

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

		// Initialise table with header
		tableData := pterm.TableData{buildTableHeader()}

		sort.Strings(mods)

		for _, filename := range mods {
			hash := filenameMapReversed[filename]

			if _, known := currentVersions[hash]; known {
				row := buildTableRow(filename, currentVersions, updates, projects, hash, AppConfig.GameVersion)
				tableData = append(tableData, row)
			}

		}

		err = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
		if err != nil {
			tui.Logger.Error("Error rendering table", "error", err)
		}

	},
}

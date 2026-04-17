package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/pterm/pterm"
)

type GorinthState struct {
	FS                  vfs.FileSystem
	Mods                []string
	FilenameMapReversed map[string]string
	CurrentVersions     map[string]modrinth.Version
	Updates             map[string]modrinth.Version
	Projects            map[string]modrinth.ModrinthProject
	Client              *modrinth.Client
}

func FetchGorinthState() (*GorinthState, error) {
	state := &GorinthState{
		FilenameMapReversed: make(map[string]string),
	}

	/*
	*	Initialize file system based on mode (local or sftp
	 */

	spinner, _ := tui.StartSpinner(fmt.Sprintf("Connecting to %s...", AppConfig.Mode))
	var err error

	switch AppConfig.Mode {
	case "local":
		state.FS, err = vfs.NewLocalFS(AppConfig.Dir)
	case "sftp":
		state.FS, err = vfs.NewSFTPFS(AppConfig.SFTPHost, AppConfig.SFTPPort, AppConfig.SFTPUser, AppConfig.SFTPPassword, AppConfig.Dir)
	default:
		tui.Logger.Fatal("Invalid mode specified", "mode", AppConfig.Mode)
	}

	if err != nil {
		spinner.Fail("Failed to connect")
		tui.Logger.Fatal("Initialization error", "error", err)
	}

	spinner.Success("Connected successfully!")

	/*
	* File system is now initialized and connected.
	 */

	/*
	* List mods and compute hashes
	 */
	spinner, _ = tui.StartSpinner("Scanning mods directory..")
	state.Mods, err = state.FS.ListMods()
	if err != nil {
		spinner.Fail("Failed to list mods")
		tui.Logger.Fatal("Error listing mods", "error", err)
	}
	spinner.Success(fmt.Sprintf("Found %d .jar files", len(state.Mods)))

	spinner, _ = tui.StartSpinner("Computing SHA-1 hashes...")

	// Store hashes in a map for later use
	hashList := make([]string, 0, len(state.Mods))

	for _, mod := range state.Mods {
		hash, err := state.FS.HashMod(mod)
		if err != nil {
			spinner.Fail(fmt.Sprintf("Failed to hash mod: %s", mod))
			tui.Logger.Error("Error hashing mod", "mod", mod, "error", err)
			continue
		}
		hashList = append(hashList, hash)
		// filenameMap[hash] = mod
		state.FilenameMapReversed[mod] = hash
	}

	// Success
	spinner.Success(fmt.Sprintf("Hashed %d mods", len(hashList)))

	// Debug output to verify it worked:
	for hash, mod := range state.FilenameMapReversed {
		tui.Logger.Debug("Hashed file", "mod", mod, "sha1", hash)
	}

	/*
	* Query Modrinth API for current versions and updates based on the computed hashes
	 */

	tui.Logger.Info("Ready to query Modrinth API", "mc_version", AppConfig.GameVersion)

	spinner, _ = tui.StartSpinner("Querying Modrinth API...")
	client := modrinth.NewClient("Frank-", "gorinth", "0.1.0")

	state.CurrentVersions, err = client.CheckVersionsFromHashes(context.Background(), hashList)

	if err != nil {
		spinner.Fail("Failed to check current versions")
		tui.Logger.Fatal("Error checking for current versions", "error", err)
	}
	state.Updates, err = client.CheckForUpdates(context.Background(), hashList, AppConfig.GameVersion, AppConfig.Loader)

	if err != nil {
		spinner.Fail("Failed to check for updates")
		tui.Logger.Fatal("Error checking for updates", "error", err)
	}

	spinner.Success("Modrinth API query successful!")

	spinner, _ = tui.StartSpinner("Fetching mod details...")
	projectIDSet := make(map[string]bool)
	for _, version := range state.CurrentVersions {
		projectIDSet[version.ProjectID] = true
	}
	for _, version := range state.Updates {
		projectIDSet[version.ProjectID] = true
	}

	var projectIDs []string
	for id := range projectIDSet {
		projectIDs = append(projectIDs, id)
	}

	state.Projects, err = client.GetProjects(projectIDs)
	if err != nil {
		spinner.Fail("Failed to fetch mod names")
		tui.Logger.Fatal("Error fetching mod names", "error", err)
	} else {
		spinner.Success("Fetched mod details successfully!")
	}

	return state, nil
}

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
	current, isKnown := currentVerMap[hash]
	update, hasUpdate := updatesMap[hash]

	displayName := truncateText(filename, 35)
	status := pterm.LightCyan("Up to date")
	currentVer := "-"
	currentMC := "-"
	newVer := "-"
	newMC := "-"
	compatibility := pterm.LightGreen("Compatible")

	// If the current version is already the latest, treat it as no update available
	if hasUpdate && isKnown && update.ID == current.ID {
		hasUpdate = false
	}

	upToDate := isKnown && !hasUpdate

	if isKnown {
		currentVer = truncateText(current.VersionNumber, 18)
		currentMC = formatMCVersions(current.GameVersions)

		if project, exists := projectsMap[current.ProjectID]; exists {
			rawTitle := truncateText(project.Title, 35)

			if upToDate {
				displayName = pterm.Italic.Sprint(rawTitle)
			} else {
				displayName = pterm.Bold.Sprint(rawTitle)
			}
		}

		supportsTarget := false
		for _, gv := range current.GameVersions {
			if gv == targetVersion {
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
			rawNewVer := truncateText(update.VersionNumber, 18)
			newVer = pterm.Bold.Sprint(rawNewVer)
			newMC = pterm.LightYellow(formatMCVersions(update.GameVersions))
			compatibility = pterm.LightGreen("Available")
		}

	}

	if !isKnown {
		displayName = truncateText(filename, 35)
		status = pterm.LightRed("Unknown")
		compatibility = pterm.LightRed("Unknown")
	}

	return []string{status, displayName, currentVer, currentMC, newVer, newMC, compatibility}
}

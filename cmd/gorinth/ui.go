package main

import (
	"fmt"
	"strings"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/pterm/pterm"
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

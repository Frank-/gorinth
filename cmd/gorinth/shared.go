package main

import (
	"context"
	"fmt"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
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
	// filenameMap := make(map[string]string)
	// filenameMapReversed := make(map[string]string)

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

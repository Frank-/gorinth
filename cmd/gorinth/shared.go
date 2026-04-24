package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
)

type GorinthState struct {
	RemoteFS            vfs.FileSystem
	WorkingFS           vfs.FileSystem
	StagingDir          string
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
	if err := state.connectFS(); err != nil {
		return nil, err
	}

	/*
	* Set up staging area for safe updates
	 */
	if err := state.setupWorkingFS(); err != nil {
		return nil, err
	}

	/*
	* List mods and compute hashes
	 */
	if err := state.computedHashes(); err != nil {
		return nil, err
	}

	/*
	* Query Modrinth API for current versions and updates based on the computed hashes
	 */
	if err := state.fetchModrinthData(); err != nil {
		return nil, err
	}

	return state, nil
}

func getFS() (vfs.FileSystem, error) {
	var fs vfs.FileSystem
	var err error
	switch AppConfig.Mode {
	case "local":
		fs, err = vfs.NewLocalFS(AppConfig.ModsDir)
	case "sftp":
		fs, err = vfs.NewSFTPFS(AppConfig.Host, AppConfig.Port, AppConfig.User, AppConfig.Password, AppConfig.ModsDir)
	default:
		tui.Logger.Fatal("Invalid mode specified", "mode", AppConfig.Mode)
	}

	return fs, err
}

func (state *GorinthState) connectFS() error {
	spinner, _ := tui.StartSpinner(fmt.Sprintf("Connecting to %s...", AppConfig.Mode))

	var err error
	state.RemoteFS, err = getFS()

	if err != nil {
		spinner.Fail("Failed to connect")
		return fmt.Errorf("initialization error: %w", err)
	}

	spinner.Success("Connected successfully!")
	return nil
}

func (state *GorinthState) setupWorkingFS() error {
	// Use direct streaming
	if AppConfig.Mode == "local" || AppConfig.Direct {
		state.WorkingFS = state.RemoteFS
		if AppConfig.Mode == "local" {
			tui.Logger.Info("Running in Local mode (bypassing staging cache)")
		} else {
			tui.Logger.Info("Running in Direct stream mode (bypassing staging cache)")
		}
		return nil
	}

	spinner, _ := tui.StartSpinner("Setting up staging area...")

	tmpDir, err := os.MkdirTemp("", "gorinth-staging-*")
	if err != nil {
		spinner.Fail("Failed to create staging area")
		return fmt.Errorf("failed to create staging area: %w", err)
	}

	state.StagingDir = tmpDir

	// Use RemoteFS to sync mods to staging area
	if err := state.RemoteFS.SyncToDir(tmpDir); err != nil {
		spinner.Fail("Failed to sync mods to staging area")
		return fmt.Errorf("failed to sync mods to staging area: %w", err)
	}

	// Set WorkingFS to a new LocalFS pointing to the staging directory
	state.WorkingFS, err = vfs.NewLocalFS(tmpDir)
	if err != nil {
		spinner.Fail("Failed to initialize staging file system")
		return fmt.Errorf("failed to initialize staging file system: %w", err)
	}

	spinner.Success("Staging area ready!")
	return nil
}

func (state *GorinthState) computedHashes() error {
	spinner, _ := tui.StartSpinner("Scanning mods directory..")

	var err error
	state.Mods, err = state.WorkingFS.ListMods()
	if err != nil {
		spinner.Fail("Failed to list mods")
		tui.Logger.Fatal("Error listing mods", "error", err)
	}
	spinner.Success(fmt.Sprintf("Found %d .jar files", len(state.Mods)))
	spinner, _ = tui.StartSpinner("Computing SHA-1 hashes...")

	hashes, err := state.WorkingFS.HashMods()

	state.FilenameMapReversed = hashes
	spinner.Success(fmt.Sprintf("Hashed %d mods", len(state.FilenameMapReversed)))

	if AppConfig.Debug {
		// Debug output for verification
		for hash, mod := range state.FilenameMapReversed {
			tui.Logger.Debug("Hashed file", "mod", mod, "sha1", hash)
		}
	}
	return err
}

func (state *GorinthState) fetchModrinthData() error {
	tui.Logger.Info("Ready to query Modrinth API", "mc_version", AppConfig.GameVersion)
	spinner, _ := tui.StartSpinner("Querying Modrinth API...")

	client := modrinth.NewClient("Frank-", "gorinth", "0.1.0")

	hashList := make([]string, 0, len(state.FilenameMapReversed))
	for _, hash := range state.FilenameMapReversed {
		hashList = append(hashList, hash)
	}

	var err error
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

	return nil
}

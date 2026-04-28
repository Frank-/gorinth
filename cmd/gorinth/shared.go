package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Frank-/gorinth/internal/modrinth"
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/Frank-/gorinth/internal/vfs"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type GorinthState struct {
	TargetFS            vfs.FileSystem
	StagingDir          string
	Mods                []string
	FilenameMapReversed map[string]string
	CurrentVersions     map[string]modrinth.Version
	Updates             map[string]modrinth.Version
	Projects            map[string]modrinth.ModrinthProject
	Client              *modrinth.Client
}

func FetchGorinthState(baseFS afero.Fs) (*GorinthState, error) {
	state := &GorinthState{
		FilenameMapReversed: make(map[string]string),
	}

	/*
	*	Initialize file system based on mode (local or sftp
	 */
	if err := state.connectFS(baseFS); err != nil {
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

func (state *GorinthState) connectFS(baseFS afero.Fs) error {
	spinner, _ := tui.StartSpinner(fmt.Sprintf("Connecting to %s...", AppConfig.Mode))

	var err error
	state.TargetFS, err = connectAndMount(baseFS)

	if err != nil {
		spinner.Fail("Failed to connect")
		return fmt.Errorf("initialization error: %w", err)
	}

	spinner.Success("Connected successfully!")
	return nil
}

func (state *GorinthState) computedHashes() error {
	spinner, _ := tui.StartSpinner("Scanning mods directory..")

	var err error
	state.Mods, err = state.TargetFS.ListMods()
	if err != nil {
		spinner.Fail("Failed to list mods")
		return fmt.Errorf("failed to list mods: %w", err)
	}
	spinner.Success(fmt.Sprintf("Found %d .jar files", len(state.Mods)))

	spinner, _ = tui.StartSpinner("Computing SHA-1 hashes...")

	hashes, err := state.TargetFS.HashMods()
	if err != nil {
		spinner.Fail("Failed to compute hashes")
		return fmt.Errorf("failed to compute mod hashes: %w", err)
	}

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
		return fmt.Errorf("failed to check current versions: %w", err)
	}

	state.Updates, err = client.CheckForUpdates(context.Background(), hashList, AppConfig.GameVersion, AppConfig.Loader)
	if err != nil {
		spinner.Fail("Failed to check for updates")
		return fmt.Errorf("failed to check for updates: %w", err)
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
		return fmt.Errorf("failed to fetch mod names: %w", err)
	} else {
		spinner.Success("Fetched mod details successfully!")
	}

	return nil
}

func WithGorinthState(action func(cmd *cobra.Command, args []string, state *GorinthState) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Create a context that we can use to manage cancellation and timeouts if needed
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		go func() {
			select {
			case <-sigCh:
				fmt.Print("\r\033[K")
				pterm.Warning.Println("\nProcess interrupted by user! Cleaning up server connections...")
				stop()
			case <-ctx.Done():
				// Normal context cancellation, no need to print anything
				return
			}
		}()

		// initialize appFS for the state fetcher
		appFS := afero.NewOsFs()

		// Safely fetch state
		state, err := FetchGorinthState(appFS)
		if err != nil {
			return err
		}

		// Guarantee cleanup of resources after action completes
		defer func() {
			if state.TargetFS != nil {
				state.TargetFS.CleanupTmpFiles()
				state.TargetFS.Close()
			}
		}()

		return action(cmd, args, state)
	}
}

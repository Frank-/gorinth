package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for updates to your Minecraft server mods without applying them.",
	Long:  `The check command allows you to see if there are any updates available for your Minecraft server mods without actually downloading or applying them. This is useful for planning updates and ensuring compatibility before making changes to your server.`,
	Run: func(cmd *cobra.Command, args []string) {
		tui.Logger.Infof("Starting Gorinth in %s mode", AppConfig.Mode)

		var state *GorinthState

		// Listen for interrupt signals to allow graceful shutdown during the update process
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		// Start a goroutine to handle interrupts and perform cleanup
		go func() {
			<-sigCh
			fmt.Print("\r\033[K")
			pterm.Warning.Println("\nProcess interrupted by user! Cleaning up server connections...")

			if state != nil && state.FS != nil {
				state.FS.Close()
				pterm.Success.Println("Cleanup complete. Exiting.")
			} else {
				tui.Logger.Warn("Gorinth state not fully initialized, skipping cleanup")
			}
			os.Exit(1)
		}()

		var err error
		state, err = FetchGorinthState()

		if err != nil {
			tui.Logger.Fatal("Failed to fetch Gorinth state", "error", err)
		}

		defer state.FS.Close()

		// Initialise table with header
		tableData := pterm.TableData{buildTableHeader()}

		var managedMods []string
		var unmanagedMods []string

		for _, filename := range state.Mods {
			hash := state.FilenameMapReversed[filename]
			if _, known := state.CurrentVersions[hash]; known {
				managedMods = append(managedMods, filename)
			} else {
				unmanagedMods = append(unmanagedMods, filename)
			}
		}

		sort.Strings(managedMods)
		sort.Strings(unmanagedMods)

		// Managed mods first
		for _, filename := range managedMods {
			hash := state.FilenameMapReversed[filename]
			row := buildTableRow(filename, state.CurrentVersions, state.Updates, state.Projects, hash, AppConfig.GameVersion)
			tableData = append(tableData, row)
		}

		if len(unmanagedMods) > 0 {
			tui.Logger.Warn(fmt.Sprintf("Found %d unmanaged mod(s) not tracked by Modrinth", len(unmanagedMods)))
			// Add a separator row before listing unmanaged mods
			tableData = append(tableData, []string{"-", "-", "-", "-", "-", "-"})
			for _, filename := range unmanagedMods {
				hash := state.FilenameMapReversed[filename]
				row := buildTableRow(filename, state.CurrentVersions, state.Updates, state.Projects, hash, AppConfig.GameVersion)
				tableData = append(tableData, row)
			}
		}

		err = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
		if err != nil {
			tui.Logger.Error("Error rendering table", "error", err)
		}

	},
}

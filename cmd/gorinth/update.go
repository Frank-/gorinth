package main

import (
	"github.com/Frank-/gorinth/internal/tui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and apply them to your Minecraft server mods.",
	Long:  `The update command checks for available updates for your Minecraft server mods and applies them automatically. This command will download the latest versions of your mods from Modrinth and replace the old versions in your server's mod directory, ensuring that your server is always running the most up-to-date mods.`,
	Run: func(cmd *cobra.Command, args []string) {
		tui.Logger.Infof("Starting Gorinth Update in %s mode", AppConfig.Mode)

		// spinner, _ := tui.StartSpinner(fmt.Sprintf("Connecting to %s...", AppConfig.Mode))
	},
}

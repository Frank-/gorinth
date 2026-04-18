package main

import (
	"fmt"
	"strings"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	ConfigFile   string `mapstructure:"config-file"`
	Mode         string `mapstructure:"mode"`
	SFTPHost     string `mapstructure:"sftp-host"`
	SFTPPort     int    `mapstructure:"sftp-port"`
	SFTPUser     string `mapstructure:"sftp-user"`
	SFTPPassword string `mapstructure:"sftp-password"`
	GameVersion  string `mapstructure:"game-version"`
	Loader       string `mapstructure:"loader"`
	Dir          string `mapstructure:"dir"`
	NoTruncate   bool   `mapstructure:"no-truncate"`
	SkipBackup   bool   `mapstructure:"skip-backup"`
	Debug        bool   `mapstructure:"debug"`
	Force        bool   `mapstructure:"force"`
}

var AppConfig Config

var rootCmd = &cobra.Command{
	Use:   "gorinth",
	Short: "A CLI tool for updating Minecraft server mods from Modrinth.",
	Long:  `Gorinth is a command-line tool that helps you keep your Minecraft server mods up to date by fetching the latest versions from Modrinth.`,
	// Run the PersistentPreRunE function before any command
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialise config
		setupConfig()

		// Unmarshal config into AppConfig struct
		if err := viper.Unmarshal(&AppConfig); err != nil {
			return err
		}

		if AppConfig.Debug {
			tui.SetDebugMode()
			tui.Logger.Debug("Debug mode enabled")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Run help
		cmd.Help()
	},
}

func init() {
	setupFlags()
	// setupConfig()

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(updateCmd)
}

func setupConfig() {
	cfgFile := viper.GetString("config-file")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigType("yaml")

	// Read from environment variables
	viper.SetEnvPrefix("GORINTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		tui.Logger.Warn(fmt.Sprintf("Could not read config file: %v", err))
	} else {
		tui.Logger.Info(fmt.Sprintf("Success: %s file loaded!", cfgFile))
	}
}

func setupFlags() {
	flags := rootCmd.PersistentFlags()

	// mode
	flags.String("mode", "sftp", "Mode of operation: `local` or `sftp`")
	flags.String("config-file", "config.yaml", "Path to the configuration file")
	// Connection
	flags.String("sftp-host", "localhost", "SFTP host for remote server")
	flags.IntP("sftp-port", "p", 22, "SFTP port for remote server")
	flags.StringP("sftp-user", "u", "user", "SFTP username for remote server")
	flags.String("sftp-password", "password", "SFTP password for remote server")

	// Directory
	flags.String("dir", ".", "Directory to store mods")

	// Minecraft version and loader
	flags.String("game-version", "1.20.4", "Minecraft version to check for updates")
	flags.String("loader", "fabric", "Mod loader to check for updates (e.g. fabric, forge)")

	// Utility
	flags.Bool("no-truncate", false, "Disable truncation of mod names in the update table for better readability")
	flags.Bool("skip-backup", false, "Skip backup creation before applying updates. Not recommended.")
	flags.BoolP("debug", "d", false, "Enable debug logging")
	flags.Bool("force", false, "Bypass safety checks and force updates. May break everything. Use with caution.")

	// Bind everything
	viper.BindPFlags(flags)

}

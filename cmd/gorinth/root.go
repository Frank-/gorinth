package main

import (
	"fmt"
	"strings"

	"github.com/Frank-/gorinth/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	Debug bool `mapstructure:"debug"`
	Mode string `mapstructure:"mode"`
	SFTPHost string `mapstructure:"sftp-host"`
	SFTPPort int `mapstructure:"sftp-port"`
	SFTPUser string `mapstructure:"sftp-user"`
	SFTPPassword string `mapstructure:"sftp-password"`
	MCVersion string `mapstructure:"mc-version"`
	Loader string `mapstructure:"loader"`
	Dir string `mapstructure:"dir"`
}

var AppConfig Config

var rootCmd = &cobra.Command{
		Use:   "gorinth",
		Short: "A CLI tool for updating Minecraft server mods from Modrinth.",
		Long:  `Gorinth is a command-line tool that helps you keep your Minecraft server mods up to date by fetching the latest versions from Modrinth.`,
		// Run the PersistentPreRunE function before any command
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
	setupConfig()

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(updateCmd)
}


func setupConfig() {
// Read from .env file if it exists
	viper.SetConfigFile("config.yaml")
	viper.SetConfigType("yaml")
	// viper.AddConfigPath(".")

	// Read from environment variables
	viper.SetEnvPrefix("GORINTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("⚠️  Notice: Could not read config file: %v\n", err)
	} else {
		fmt.Println("✅ Success: config.yaml file loaded!")
	}
}

func setupFlags() {
	flags := rootCmd.PersistentFlags()

	flags.BoolP("debug", "d", false, "Enable debug logging")
	// mode
	flags.String("mode", "local", "Mode of operation: `local` or `sftp`")
	// Connection
	flags.String("sftp-host", "localhost", "SFTP host for remote server")
	flags.IntP("sftp-port", "p", 22, "SFTP port for remote server")
	flags.StringP("sftp-user", "u", "user", "SFTP username for remote server")
	flags.String("sftp-password", "password", "SFTP password for remote server")

	// Directory
	flags.String("dir", ".", "Directory to store mods")

	// Minecraft version and loader
	flags.String("mc-version", "1.20.4", "Minecraft version to check for updates")
	flags.String("loader", "fabric", "Mod loader to check for updates (e.g. fabric, forge)")

	// Bind everything
	viper.BindPFlags(flags)

}
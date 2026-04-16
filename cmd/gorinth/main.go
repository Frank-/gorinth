package main

import (
	"os"

	"github.com/Frank-/gorinth/internal/tui"
)


func main() {
	if err := rootCmd.Execute(); err != nil {
		tui.Logger.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
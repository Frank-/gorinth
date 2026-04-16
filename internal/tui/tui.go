package tui

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pterm/pterm"
)


var Logger *log.Logger

func init() {
	// Init charm logger
	Logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller: false,
		ReportTimestamp: true,
		TimeFormat: time.TimeOnly,
		Prefix: "gorinth",
		Level: log.InfoLevel,
	})
}

func StartSpinner(text string) (*pterm.SpinnerPrinter, error) {
	spinner, err := pterm.DefaultSpinner.
									WithRemoveWhenDone(true).
									WithText(text).
									Start()
	return spinner, err
}

func SetDebugMode() {
	Logger.SetLevel(log.DebugLevel)
}
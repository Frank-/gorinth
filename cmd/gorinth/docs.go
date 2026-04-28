package main

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation for gorinth",
	Hidden: true, // We don't want users to see this in the help
	Run: func(cmd *cobra.Command, args []string) {
		err := doc.GenMarkdownTree(rootCmd, "./docs")
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	// Add the docs command to rootCmd
	rootCmd.AddCommand(docsCmd)
}

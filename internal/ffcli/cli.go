// Package ffcli is a simple command line interface for frameforge
package ffcli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ffcli",
	Short: "ffcli is a simple command line interface for frameforge",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// Package ffcli is a simple command line interface for frameforge
package ffcli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "frameforge",
	Short: "frameforge is the command line interface for the frameforge engine",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

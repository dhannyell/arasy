// Package cli is a simple command line interface for arasy
package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "arasy",
	Short: "arasy is the command line interface for the arasy engine",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(exitCodeFor(err))
	}
}

// exitCoder is an error that names the exit status it should produce. It
// exists for `arasy run`: a game that exits 3 has to leave the CLI exiting 3,
// or every wrapper script sees the same 1 no matter what the game reported.
type exitCoder interface{ ExitCode() int }

// exitCodeFor resolves the status an error should exit with. Anything that
// does not name one is an ordinary CLI failure and stays 1 — including an
// error that names 0, which would otherwise turn a failure into a success.
func exitCodeFor(err error) int {
	var coder exitCoder
	if errors.As(err, &coder) {
		if code := coder.ExitCode(); code != 0 {
			return code
		}
	}
	return 1
}

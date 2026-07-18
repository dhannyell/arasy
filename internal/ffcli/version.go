package ffcli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is injected at release time via -ldflags.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the frameforge CLI version",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
		return err
	},
}

func init() { rootCmd.AddCommand(versionCmd) }

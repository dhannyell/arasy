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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), version)
	},
}

func init() { rootCmd.AddCommand(versionCmd) }

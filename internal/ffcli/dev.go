package ffcli

import (
	"fmt"
	"os"

	"github.com/dhannyell/frameforge/internal/wasmserve"
	"github.com/spf13/cobra"
)

var platform string
var projectPath string
var address string
var wasmExecPath string
var execPath string

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "build and start a development game",
	Long:  `Start and build a game in development mode`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("platform: %s\n", platform)

		switch platform {
		case "wasm":
			fmt.Printf("Starting wasm server")
			if projectPath == "" {
				currentPath, err := os.Getwd()
				if err != nil {
					os.Exit(1)
				}
				projectPath = currentPath
			}
			wasmserve.Start(wasmserve.NewServerOptions(projectPath, "", "app.wasm", execPath, address))
		default:
			fmt.Printf("Error: Unknown '%s' platform.", platform)
		}
	},
}

func init() {
	devCmd.Flags().StringVarP(&platform, "platform", "p", "wasm", "platform to development build")
	devCmd.Flags().StringVarP(&projectPath, "project-path", "s", "", "project path")
	devCmd.Flags().StringVarP(&address, "address", "a", "127.0.0.1:8080", "server address")
	devCmd.Flags().StringVarP(&wasmExecPath, "wasm-exec-path", "w", ".", "path to wasm executable")
	devCmd.Flags().StringVarP(&execPath, "exec-path", "e", "", "path to executable")

	err := devCmd.RegisterFlagCompletionFunc("platform", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		options := []string{"wasm"}

		return options, cobra.ShellCompDirectiveDefault
	})
	if err != nil {
		os.Exit(1)
	}

	rootCmd.AddCommand(devCmd)
}

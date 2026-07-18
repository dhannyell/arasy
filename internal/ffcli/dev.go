package ffcli

import (
	"fmt"
	"os"

	"github.com/dhannyell/frameforge/internal/wasmserve"
	"github.com/spf13/cobra"
)

var (
	devPlatform string
	devAddress  string
	devTitle    string
)

var devCmd = &cobra.Command{
	Use:   "dev [package]",
	Short: "build and serve a WASM game for development",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := supportedPlatform(devPlatform); err != nil {
			return err
		}
		pkg := packageArg(args)
		title := devTitle
		if title == "" {
			title = defaultTitle(pkg)
		}
		dir, err := os.MkdirTemp("", "frameforge-dev-*")
		if err != nil {
			return err
		}
		if err := rebuildWASM(pkg, dir, title); err != nil {
			if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "initial build failed; reload the browser after fixing it:\n%s\n", err); writeErr != nil {
				return writeErr
			}
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "serving %s at http://%s; reload the browser to rebuild\n", pkg, devAddress); err != nil {
			return err
		}
		wasmserve.Start(&wasmserve.ServerOptions{
			Dir: dir, Title: title, WASMFile: "app.wasm", Addr: devAddress,
			Rebuild: func() error { return rebuildWASM(pkg, dir, title) },
		})
		return nil
	},
}

func init() {
	devCmd.Flags().StringVarP(&devPlatform, "platform", "p", "wasm", "target platform")
	devCmd.Flags().StringVarP(&devAddress, "address", "a", "127.0.0.1:8080", "server address")
	devCmd.Flags().StringVar(&devTitle, "title", "", "HTML page title")
	if err := devCmd.RegisterFlagCompletionFunc("platform", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"wasm"}, cobra.ShellCompDirectiveDefault
	}); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(devCmd)
}

package ffcli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dhannyell/frameforge/internal/wasmserve"
	"github.com/spf13/cobra"
)

var (
	serveAddress string
	serveTitle   string
	serveWASM    string
)

var serveCmd = &cobra.Command{
	Use:   "serve [dir]",
	Short: "serve a completed WASM build",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "dist/wasm"
		if len(args) == 1 {
			dir = args[0]
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("build directory %q does not exist", dir)
		}
		if _, err := os.Stat(filepath.Join(dir, serveWASM)); err != nil {
			return fmt.Errorf("build directory %q does not contain %s", dir, serveWASM)
		}
		wasmserve.Start(&wasmserve.ServerOptions{
			Dir: dir, Title: serveTitle, WASMFile: serveWASM, Addr: serveAddress,
		})
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveAddress, "address", "a", "127.0.0.1:8080", "server address")
	serveCmd.Flags().StringVar(&serveTitle, "title", "", "HTML page title")
	serveCmd.Flags().StringVar(&serveWASM, "wasm", "app.wasm", "WASM file name")
	rootCmd.AddCommand(serveCmd)
}

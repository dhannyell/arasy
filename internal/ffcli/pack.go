package ffcli

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	packOutput string
	packTitle  string
)

var packCmd = &cobra.Command{
	Use:   "pack [package]",
	Short: "build a WASM game and package it as a ZIP",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pkg := packageArg(args)
		title := packTitle
		if title == "" {
			title = defaultTitle(pkg)
		}
		output := packOutput
		if output == "" {
			output = filepath.Join("dist", defaultTitle(pkg)+".zip")
		}
		staging, err := os.MkdirTemp("", "frameforge-pack-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		if err := buildWASM(pkg, staging, title, true, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return err
		}
		return zipBuild(output, staging)
	},
}

func init() {
	packCmd.Flags().StringVarP(&packOutput, "output", "o", "", "output ZIP path")
	packCmd.Flags().StringVar(&packTitle, "title", "", "HTML page title")
	rootCmd.AddCommand(packCmd)
}

func zipBuild(output, dir string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"index.html", "app.wasm", "wasm_exec.js"} {
		in, openErr := os.Open(filepath.Join(dir, name))
		if openErr != nil {
			zw.Close()
			f.Close()
			return openErr
		}
		entry, createErr := zw.Create(name)
		if createErr == nil {
			_, createErr = io.Copy(entry, in)
		}
		in.Close()
		if createErr != nil {
			zw.Close()
			f.Close()
			return createErr
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

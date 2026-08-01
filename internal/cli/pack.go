package cli

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dhannyell/arasy/internal/wasmserve"
	"github.com/spf13/cobra"
)

var (
	packPlatform      string
	packOutput        string
	packTitle         string
	packNoDefaultFont bool
	packDevTools      bool
	packMode          string
	packAssets        string
	packBoot          webBootFlags
)

var packCmd = &cobra.Command{
	Use:   "pack [package]",
	Short: "build a game and package it as a ZIP",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := supportedPlatform(packPlatform); err != nil {
			return err
		}
		if packPlatform == platformNative {
			return runPackNative(cmd, args)
		}
		pkg := packageArg(args)
		title := packTitle
		if title == "" {
			title = defaultTitle(pkg)
		}
		output := packOutput
		if output == "" {
			output = filepath.Join("dist", defaultTitle(pkg)+".zip")
		}
		boot, err := packBoot.options()
		if err != nil {
			return err
		}
		staging, err := os.MkdirTemp("", "Arasy-pack-*")
		if err != nil {
			return err
		}
		defer func() { _ = removeTempDir(staging) }()
		stopInterruptCleanup := watchTempDirInterrupt(staging)
		defer stopInterruptCleanup()
		if err := buildWASM(pkg, staging, title, wasmBuildOptions{
			Release:       true,
			NoDefaultFont: packNoDefaultFont,
			Boot:          boot,
		}, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return err
		}
		return zipBuild(output, staging)
	},
}

// runPackNative stages a native build and zips the whole staging directory:
// the archive is the depot layout of D7 — the executable and, in bundle mode,
// the assets beside it.
func runPackNative(cmd *cobra.Command, args []string) error {
	if err := rejectWebOnlyFlags(cmd, "pack"); err != nil {
		return err
	}
	pkg := nativePackageArg(args)
	output := packOutput
	if output == "" {
		output = filepath.Join("dist", nativeArchiveName(pkg))
	}
	staging, err := os.MkdirTemp("", "Arasy-pack-*")
	if err != nil {
		return err
	}
	defer func() { _ = removeTempDir(staging) }()
	stopInterruptCleanup := watchTempDirInterrupt(staging)
	defer stopInterruptCleanup()
	if err := buildNative(pkg, staging, nativeBuildOptions{
		Release:       true,
		NoDefaultFont: packNoDefaultFont,
		DevTools:      packDevTools,
		Mode:          packMode,
		AssetsDir:     packAssets,
	}, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		return err
	}
	if err := zipDir(output, staging); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
	return err
}

// nativeArchiveName names the archive after the product and the host it was
// built for. There is no cross-compilation, so the host is the target.
func nativeArchiveName(pkg string) string {
	return fmt.Sprintf("%s-%s-%s.zip", nativeName(pkg), runtime.GOOS, runtime.GOARCH)
}

func init() {
	packCmd.Flags().StringVarP(&packPlatform, "platform", "p", platformWASM, "target platform (wasm or native)")
	packCmd.Flags().StringVarP(&packOutput, "output", "o", "", "output ZIP path")
	packCmd.Flags().StringVar(&packTitle, "title", "", "HTML page title")
	packCmd.Flags().BoolVar(&packNoDefaultFont, "no-default-font", false, "exclude the embedded default font (~140 KB); FontID 0 falls back to the built-in 7x13 bitmap")
	packCmd.Flags().BoolVar(&packDevTools, "devtools", false, "include the dev panel (arr_dev tag); release builds leave it off")
	packCmd.Flags().StringVar(&packMode, "mode", nativeModePortable, "native layout: portable (single executable) or bundle (executable plus an assets directory)")
	packCmd.Flags().StringVar(&packAssets, "assets", "", "native bundle mode: directory copied next to the executable as assets/")
	packBoot.bind(packCmd)
	registerPlatformCompletion(packCmd)
	rootCmd.AddCommand(packCmd)
}

// zipDir archives every file under dir with paths relative to dir, so the
// archive unpacks straight into the layout the game expects.
func zipDir(output, dir string) (err error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	defer func() {
		err = errors.Join(err, zw.Close(), f.Close())
	}()
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		in, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		zipEntry, createErr := zw.Create(filepath.ToSlash(rel))
		if createErr == nil {
			_, createErr = io.Copy(zipEntry, in)
		}
		closeErr := in.Close()
		if createErr != nil {
			return createErr
		}
		return closeErr
	})
}

func zipBuild(output, dir string) (err error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	defer func() {
		err = errors.Join(err, zw.Close(), f.Close())
	}()
	for _, name := range []string{
		"index.html", "app.wasm", "wasm_exec.js",
		wasmserve.PosterLandscapeFile, wasmserve.PosterStackedFile,
	} {
		in, openErr := os.Open(filepath.Join(dir, name))
		if openErr != nil {
			return openErr
		}
		entry, createErr := zw.Create(name)
		if createErr == nil {
			_, createErr = io.Copy(entry, in)
		}
		closeErr := in.Close()
		if createErr != nil {
			return createErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

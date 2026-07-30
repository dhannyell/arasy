package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dhannyell/arasy/internal/wasmserve"
	"github.com/spf13/cobra"
)

var (
	buildPlatform      string
	buildOutput        string
	buildTitle         string
	buildNoDefaultFont bool
)

var buildCmd = &cobra.Command{
	Use:   "build [package]",
	Short: "build a WASM game for static hosting",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := supportedPlatform(buildPlatform); err != nil {
			return err
		}
		pkg := packageArg(args)
		title := buildTitle
		if title == "" {
			title = defaultTitle(pkg)
		}
		return buildWASM(pkg, buildOutput, title, true, buildNoDefaultFont, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func init() {
	buildCmd.Flags().StringVarP(&buildPlatform, "platform", "p", "wasm", "target platform")
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "dist/wasm", "output directory")
	buildCmd.Flags().StringVar(&buildTitle, "title", "", "HTML page title")
	buildCmd.Flags().BoolVar(&buildNoDefaultFont, "no-default-font", false, "exclude the embedded default font (~140 KB); FontID 0 falls back to the built-in 7x13 bitmap")
	rootCmd.AddCommand(buildCmd)
}

func supportedPlatform(platform string) error {
	if platform != "wasm" {
		return fmt.Errorf("platform %q not supported yet", platform)
	}
	return nil
}

func packageArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	if info, err := os.Stat("wasm"); err == nil && info.IsDir() {
		return "./wasm"
	}
	return "."
}

func defaultTitle(pkg string) string {
	clean := filepath.Clean(pkg)
	if clean == "." {
		if cwd, err := os.Getwd(); err == nil {
			return filepath.Base(cwd)
		}
	}
	return filepath.Base(clean)
}

func buildWASM(pkg, outDir, title string, release, noDefaultFont bool, stdout, stderr io.Writer) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	cmd := exec.Command("go", wasmBuildArgs(pkg, outDir, release, noDefaultFont)...)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w", pkg, err)
	}
	wasmExec, err := wasmserve.DefaultWASMExecPath()
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(outDir, "wasm_exec.js"), wasmExec); err != nil {
		return err
	}
	shell, err := wasmserve.ShellHTML(title, "app.wasm")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), shell, 0o644); err != nil {
		return fmt.Errorf("write index.html: %w", err)
	}
	if err := wasmserve.WritePosterAssets(outDir); err != nil {
		return err
	}
	return nil
}

// wasmBuildArgs assembles the go build command line for a WASM artifact.
func wasmBuildArgs(pkg, outDir string, release, noDefaultFont bool) []string {
	args := []string{"build", "-o", filepath.Join(outDir, "app.wasm"), "-trimpath", "-buildvcs=false"}
	if release {
		args = append(args, "-ldflags", "-s -w")
	}
	if noDefaultFont {
		args = append(args, "-tags", "arr_no_default_font")
	}
	return append(args, pkg)
}

func rebuildWASM(pkg, outDir, title string) error {
	var output strings.Builder
	if err := buildWASM(pkg, outDir, title, false, false, &output, &output); err != nil {
		if output.Len() > 0 {
			return fmt.Errorf("%s", output.String())
		}
		return err
	}
	return nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	out, err := os.Create(dst)
	if err != nil {
		return errors.Join(fmt.Errorf("create %s: %w", dst, err), in.Close())
	}
	_, copyErr := io.Copy(out, in)
	inCloseErr := in.Close()
	outCloseErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if inCloseErr != nil {
		return inCloseErr
	}
	return outCloseErr
}

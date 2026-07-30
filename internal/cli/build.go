package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	buildBoot          webBootFlags
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
		boot, err := buildBoot.options()
		if err != nil {
			return err
		}
		return buildWASM(pkg, buildOutput, title, wasmBuildOptions{
			Release:       true,
			NoDefaultFont: buildNoDefaultFont,
			Boot:          boot,
		}, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func init() {
	buildCmd.Flags().StringVarP(&buildPlatform, "platform", "p", "wasm", "target platform")
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "dist/wasm", "output directory")
	buildCmd.Flags().StringVar(&buildTitle, "title", "", "HTML page title")
	buildCmd.Flags().BoolVar(&buildNoDefaultFont, "no-default-font", false, "exclude the embedded default font (~140 KB); FontID 0 falls back to the built-in 7x13 bitmap")
	buildBoot.bind(buildCmd)
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

type wasmBuildOptions struct {
	Release       bool
	NoDefaultFont bool
	Boot          wasmserve.BootOptions
}

func buildWASM(pkg, outDir, title string, opts wasmBuildOptions, stdout, stderr io.Writer) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	// Build beside the target and swap only when the bytes changed: rebuilds
	// that produce identical output preserve app.wasm's mtime, so the
	// browser's conditional request revalidates to a cheap 304 instead of
	// re-downloading the whole binary.
	tmp, err := os.CreateTemp(outDir, ".app-*.wasm")
	if err != nil {
		return fmt.Errorf("create temporary wasm output: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary wasm output: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }() // no-op after a successful swap

	cmd := exec.Command("go", wasmBuildArgs(pkg, tmpPath, opts.Release, opts.NoDefaultFont)...)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w", pkg, err)
	}
	same, err := sameFileContent(tmpPath, filepath.Join(outDir, "app.wasm"))
	if err != nil {
		return err
	}
	if !same {
		if err := os.Rename(tmpPath, filepath.Join(outDir, "app.wasm")); err != nil {
			return fmt.Errorf("replace app.wasm: %w", err)
		}
	}
	wasmExec, err := wasmserve.DefaultWASMExecPath()
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(outDir, "wasm_exec.js"), wasmExec); err != nil {
		return err
	}
	shell, err := wasmserve.ShellHTML(title, "app.wasm", opts.Boot)
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
func wasmBuildArgs(pkg, out string, release, noDefaultFont bool) []string {
	args := []string{"build", "-o", out, "-trimpath", "-buildvcs=false"}
	if release {
		args = append(args, "-ldflags", "-s -w")
	}
	if noDefaultFont {
		args = append(args, "-tags", "arr_no_default_font")
	}
	return append(args, pkg)
}

// sameFileContent reports whether two files hold identical bytes. A missing
// destination is simply "not the same".
func sameFileContent(candidate, current string) (bool, error) {
	fa, err := os.Open(candidate)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", candidate, err)
	}
	defer func() { _ = fa.Close() }()
	fb, err := os.Open(current)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open %s: %w", current, err)
	}
	defer func() { _ = fb.Close() }()

	faInfo, err := fa.Stat()
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", candidate, err)
	}
	fbInfo, err := fb.Stat()
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", current, err)
	}
	if faInfo.Size() != fbInfo.Size() {
		return false, nil
	}

	ha, hb := sha256.New(), sha256.New()
	if _, err := io.Copy(ha, fa); err != nil {
		return false, fmt.Errorf("hash %s: %w", candidate, err)
	}
	if _, err := io.Copy(hb, fb); err != nil {
		return false, fmt.Errorf("hash %s: %w", current, err)
	}
	return bytes.Equal(ha.Sum(nil), hb.Sum(nil)), nil
}

func rebuildWASM(pkg, outDir, title string, boot wasmserve.BootOptions) error {
	var output strings.Builder
	if err := buildWASM(pkg, outDir, title, wasmBuildOptions{Boot: boot}, &output, &output); err != nil {
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

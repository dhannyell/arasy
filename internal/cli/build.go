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
	"runtime"
	"strings"

	"github.com/dhannyell/arasy/internal/wasmserve"
	"github.com/spf13/cobra"
)

var (
	buildPlatform      string
	buildOutput        string
	buildTitle         string
	buildNoDefaultFont bool
	buildDevTools      bool
	buildMode          string
	buildAssets        string
	buildConsole       bool
	buildBoot          webBootFlags
)

var buildCmd = &cobra.Command{
	Use:   "build [package]",
	Short: "build a game for distribution: a WASM directory for static hosting, or a native executable",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := supportedPlatform(buildPlatform); err != nil {
			return err
		}
		if buildPlatform == platformNative {
			return runBuildNative(cmd, args)
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
			DevTools:      buildDevTools,
			Boot:          boot,
		}, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func runBuildNative(cmd *cobra.Command, args []string) error {
	if err := rejectWebOnlyFlags(cmd, "build"); err != nil {
		return err
	}
	pkg := nativePackageArg(args)
	outDir := buildOutput
	if !cmd.Flags().Changed("output") {
		outDir = filepath.Join("dist", platformNative)
	}
	if err := buildNative(pkg, outDir, nativeBuildOptions{
		Release:       true,
		NoDefaultFont: buildNoDefaultFont,
		DevTools:      buildDevTools,
		Mode:          buildMode,
		AssetsDir:     buildAssets,
		Console:       buildConsole,
	}, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		return err
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "built %s into %s\n", pkg, outDir)
	return err
}

func init() {
	buildCmd.Flags().StringVarP(&buildPlatform, "platform", "p", platformWASM, "target platform (wasm or native)")
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "dist/wasm", "output directory (native builds default to dist/native)")
	buildCmd.Flags().StringVar(&buildTitle, "title", "", "HTML page title")
	buildCmd.Flags().BoolVar(&buildNoDefaultFont, "no-default-font", false, "exclude the embedded default font (~140 KB); FontID 0 falls back to the built-in 7x13 bitmap")
	buildCmd.Flags().BoolVar(&buildDevTools, "devtools", false, "include the dev panel (arr_dev tag); release builds leave it off")
	buildCmd.Flags().StringVar(&buildMode, "mode", nativeModePortable, "native layout: portable (single executable) or bundle (executable plus an assets directory)")
	buildCmd.Flags().StringVar(&buildAssets, "assets", "", "native bundle mode: directory copied next to the executable as assets/")
	buildCmd.Flags().BoolVar(&buildConsole, "console", false, "native Windows release: keep the console window (releases link into the GUI subsystem)")
	buildBoot.bind(buildCmd)
	registerPlatformCompletion(buildCmd)
	rootCmd.AddCommand(buildCmd)
}

const (
	platformWASM   = "wasm"
	platformNative = "native"
)

func supportedPlatform(platform string) error {
	switch platform {
	case platformWASM, platformNative:
		return nil
	default:
		return fmt.Errorf("platform %q not supported yet", platform)
	}
}

// registerPlatformCompletion offers the platforms a command actually accepts.
func registerPlatformCompletion(cmd *cobra.Command) {
	err := cmd.RegisterFlagCompletionFunc("platform", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{platformWASM, platformNative}, cobra.ShellCompDirectiveDefault
	})
	if err != nil {
		panic(err)
	}
}

// webOnlyFlags shape the HTML shell and its boot sequence; a native build has
// neither, so passing one is a mistake worth reporting rather than ignoring.
var webOnlyFlags = []string{"title", "no-loading", "no-splash", "no-engine-splash", "streaming-compile", "boot-background"}

func rejectWebOnlyFlags(cmd *cobra.Command, command string) error {
	for _, name := range webOnlyFlags {
		flag := cmd.Flags().Lookup(name)
		if flag != nil && flag.Changed {
			return fmt.Errorf("%s -p native: --%s is a wasm-only flag", command, name)
		}
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

// nativePackageArg mirrors packageArg for the native entry point, which
// conventionally lives in a native/ subdirectory of the game.
func nativePackageArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	if info, err := os.Stat(platformNative); err == nil && info.IsDir() {
		return "./native"
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
	// DevTools compiles the dev panel in (the arr_dev tag): the development
	// server always sets it, and arasy build sets it with --devtools. A
	// default build is the release build and does not carry the panel.
	DevTools bool
	Boot     wasmserve.BootOptions
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

	cmd := exec.Command("go", wasmBuildArgs(pkg, tmpPath, opts.Release, opts.NoDefaultFont, opts.DevTools)...)
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
func wasmBuildArgs(pkg, out string, release, noDefaultFont, devTools bool) []string {
	args := []string{"build", "-o", out, "-trimpath", "-buildvcs=false"}
	if release {
		args = append(args, "-ldflags", "-s -w")
	}
	var tags []string
	if noDefaultFont {
		tags = append(tags, "arr_no_default_font")
	}
	if devTools {
		tags = append(tags, "arr_dev")
	}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	return append(args, pkg)
}

const (
	// nativeModePortable ships one executable with the assets compiled into
	// it: the right shape for a jam entry, a demo or a bug-report attachment.
	nativeModePortable = "portable"
	// nativeModeBundle ships the executable with an assets directory beside
	// it. It does not make the executable smaller — the game decides whether
	// it still embeds a copy. What it buys is cheap patching (a store's delta
	// updates only ship the files that changed) and mods.
	nativeModeBundle = "bundle"
)

type nativeBuildOptions struct {
	Release       bool
	NoDefaultFont bool
	// DevTools compiles the dev panel in (the arr_dev tag), matching the
	// wasm build. A default build is the release build and leaves it out.
	DevTools bool
	// Mode is nativeModePortable or nativeModeBundle.
	Mode string
	// AssetsDir is copied next to the executable as assets/ in bundle mode.
	AssetsDir string
	// Console keeps the console window on a Windows release build. Releases
	// link into the GUI subsystem by default so a double-click does not flash
	// a black console; the game's log file is what keeps that debuggable
	// (see native.NewLogger).
	Console bool
}

func (o nativeBuildOptions) validate() error {
	switch o.Mode {
	case nativeModePortable:
		if o.AssetsDir != "" {
			return fmt.Errorf("--assets applies to --mode bundle; portable builds carry their assets inside the executable")
		}
	case nativeModeBundle:
		if o.AssetsDir == "" {
			return fmt.Errorf("--mode bundle needs --assets <dir>: the directory to copy next to the executable")
		}
	default:
		return fmt.Errorf("mode %q not supported: use %s or %s", o.Mode, nativeModePortable, nativeModeBundle)
	}
	return nil
}

// buildNative compiles a native game into outDir. It always targets the host:
// the native host needs cgo, and cross-compiling cgo needs the target's C
// toolchain, so the release matrix is a matrix of runners, not of GOOS values.
func buildNative(pkg, outDir string, opts nativeBuildOptions, stdout, stderr io.Writer) error {
	if err := opts.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	binary := filepath.Join(outDir, nativeBinaryName(pkg))
	cmd := exec.Command("go", nativeBuildArgs(pkg, binary, opts)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w", pkg, err)
	}
	if opts.Mode == nativeModeBundle {
		if err := copyDir(filepath.Join(outDir, "assets"), opts.AssetsDir); err != nil {
			return err
		}
	}
	return nil
}

// nativeBuildArgs assembles the go build command line for a native artifact.
func nativeBuildArgs(pkg, out string, opts nativeBuildOptions) []string {
	args := []string{"build", "-o", out, "-trimpath", "-buildvcs=false"}
	if opts.Release {
		args = append(args, "-ldflags", nativeLDFlags(opts.Console))
	}
	var tags []string
	if opts.NoDefaultFont {
		tags = append(tags, "arr_no_default_font")
	}
	if opts.DevTools {
		tags = append(tags, "arr_dev")
	}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	return append(args, pkg)
}

// nativeLDFlags picks the linker flags for a release build. On Windows the
// default is the GUI subsystem, so double-clicking the game does not flash a
// console window; -H=windowsgui also discards stdout and stderr, which is why
// a released game logs to a file (native.NewLogger).
func nativeLDFlags(console bool) string {
	if runtime.GOOS == "windows" && !console {
		return "-H=windowsgui -s -w"
	}
	return "-s -w"
}

// nativeName derives the product name from the package path. A native entry
// point conventionally lives in a native/ subdirectory of the game, so the
// directory above it — not the literal "native" — is the name a player sees.
func nativeName(pkg string) string {
	clean := filepath.Clean(pkg)
	// Resolve against the working directory so "." and "./native" name the
	// game rather than themselves.
	if !filepath.IsAbs(clean) {
		if cwd, err := os.Getwd(); err == nil {
			clean = filepath.Join(cwd, clean)
		}
	}
	name := filepath.Base(clean)
	if name != platformNative {
		return name
	}
	if parent := filepath.Base(filepath.Dir(clean)); parent != "." && parent != string(filepath.Separator) {
		return parent
	}
	return name
}

func nativeBinaryName(pkg string) string {
	name := nativeName(pkg)
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// copyDir copies src onto dst recursively, preserving the tree shape. The
// bundle layout is loose files by design: that is what keeps delta patching
// cheap and mods possible.
func copyDir(dst, src string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("read assets directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("assets path %s is not a directory", src)
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(target, path)
	})
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
	if err := buildWASM(pkg, outDir, title, wasmBuildOptions{Boot: boot, DevTools: true}, &output, &output); err != nil {
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

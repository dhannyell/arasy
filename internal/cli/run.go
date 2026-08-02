package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	runNoDefaultFont bool
	runDevTools      bool
)

var runCmd = &cobra.Command{
	Use:   "run [package] [-- game-args]",
	Short: "build a native game and run it",
	Long: `Build a native game into a temporary directory and run it.

Everything after -- is passed to the game rather than to arasy.

The game inherits this console, so Ctrl+C reaches it directly. The build is
a development build: it carries the dev panel unless --devtools=false, and it
is not stripped. Use "arasy build -p native" for something to hand out.`,
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		pkgArgs, gameArgs := splitRunArgs(args, cmd.ArgsLenAtDash())
		if len(pkgArgs) > 1 {
			return fmt.Errorf("run takes at most one package; pass game arguments after --")
		}
		return runNative(cmd, nativePackageArg(pkgArgs), gameArgs, nativeBuildOptions{
			NoDefaultFont: runNoDefaultFont,
			DevTools:      runDevTools,
			Mode:          nativeModePortable,
		})
	},
}

func init() {
	runCmd.Flags().BoolVar(&runNoDefaultFont, "no-default-font", false, "exclude the embedded default font (~140 KB); FontID 0 falls back to the built-in 7x13 bitmap")
	runCmd.Flags().BoolVar(&runDevTools, "devtools", true, "include the dev panel (arr_dev tag); pass --devtools=false to run what a player would get")
	rootCmd.AddCommand(runCmd)
}

// splitRunArgs separates the package from the arguments meant for the game.
// Cobra reports where -- was; without it every argument is ours.
func splitRunArgs(args []string, argsLenAtDash int) (pkgArgs, gameArgs []string) {
	if argsLenAtDash < 0 {
		return args, nil
	}
	return args[:argsLenAtDash], args[argsLenAtDash:]
}

// runNative builds the package into a temporary directory and executes it.
//
// Building and executing beats `go run` for an interactive game: `go run`
// leaves a parent process between the console and the game, and on Windows it
// does not reliably forward Ctrl+C to the child. A binary started directly
// shares this console's process group, so the interrupt arrives at the game
// itself with nothing to forward.
func runNative(cmd *cobra.Command, pkg string, gameArgs []string, opts nativeBuildOptions) error {
	if err := requireNativeHost(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "arasy-run-*")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := removeTempDir(dir); cleanupErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove temporary build directory %q: %v\n", dir, cleanupErr)
		}
	}()
	stopInterruptCleanup := watchTempDirInterrupt(dir)
	defer stopInterruptCleanup()

	binary := filepath.Join(dir, nativeBinaryName(pkg))
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "building %s\n", pkg); err != nil {
		return err
	}
	if err := buildNative(pkg, dir, opts, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		return err
	}
	return runGameBinary(binary, gameArgs, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

// runGameBinary executes a built game with stdio inherited from the caller.
func runGameBinary(binary string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	game := exec.Command(binary, args...)
	game.Stdin = stdin
	game.Stdout = stdout
	game.Stderr = stderr
	if err := game.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return gameExitError{binary: filepath.Base(binary), code: exitErr.ExitCode()}
		}
		return fmt.Errorf("run %s: %w", binary, err)
	}
	return nil
}

// gameExitError carries a game's own exit status out through cli.Execute.
type gameExitError struct {
	binary string
	code   int
}

func (e gameExitError) Error() string { return fmt.Sprintf("%s exited with code %d", e.binary, e.code) }
func (e gameExitError) ExitCode() int { return e.code }

// requireNativeHost rejects a host the native build cannot target. There is no
// cross-compilation to fall back on, so this is a hard stop rather than a
// warning. It does not probe for a C toolchain: a failed build reports that
// better than a guess would, and `arasy doctor` is where diagnosis lives.
func requireNativeHost() error {
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
	default:
		return fmt.Errorf("the native host does not support %s; it builds on windows, linux and darwin", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("the native host does not support %s; it builds on amd64 and arm64", runtime.GOARCH)
	}
	return nil
}

// runDevNative is the native development loop: build, run, and when the game
// exits offer to do it again. There is no file watcher — the rebuild is an
// explicit gesture, matching what a browser reload does for the wasm host.
//
// Rebuilding only after the game exits is not a simplification: Windows locks
// a running executable, so a rebuild while it is up would fail to write. The
// loop's shape avoids the problem instead of working around it.
func runDevNative(cmd *cobra.Command, pkg string, opts nativeBuildOptions) error {
	if err := requireNativeHost(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "arasy-dev-native-*")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := removeTempDir(dir); cleanupErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove temporary build directory %q: %v\n", dir, cleanupErr)
		}
	}()
	stopInterruptCleanup := watchTempDirInterrupt(dir)
	defer stopInterruptCleanup()

	binary := filepath.Join(dir, nativeBinaryName(pkg))
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	prompt := bufio.NewReader(cmd.InOrStdin())

	for {
		if buildErr := buildNative(pkg, dir, opts, out, errOut); buildErr != nil {
			if _, err := fmt.Fprintf(errOut, "build failed: %v\n", buildErr); err != nil {
				return err
			}
		} else if runErr := runGameBinary(binary, nil, nil, out, errOut); runErr != nil {
			// The game's own failure is news, not the loop's: report it and
			// offer the same prompt, because a crash is exactly when someone
			// wants to fix a line and try again.
			if _, err := fmt.Fprintf(errOut, "%v\n", runErr); err != nil {
				return err
			}
		}
		again, err := promptRebuild(prompt, out)
		if err != nil || !again {
			return err
		}
	}
}

// promptRebuild asks for the rebuild gesture and reports whether the loop
// should go around again. EOF — a closed or non-interactive stdin — ends it
// cleanly, so a script or a CI job never hangs here waiting for a key.
func promptRebuild(in *bufio.Reader, out io.Writer) (bool, error) {
	if _, err := fmt.Fprintf(out, "press Enter to rebuild and relaunch (Ctrl+C to quit)\n"); err != nil {
		return false, err
	}
	_, err := in.ReadString('\n')
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

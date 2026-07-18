package ffcli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dhannyell/frameforge/internal/wasmserve"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor [package]",
	Short: "check the frameforge development environment",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		failed := false
		version, err := exec.Command("go", "version").Output()
		if err != nil || !strings.HasPrefix(strings.TrimSpace(string(version)), "go version ") {
			if err := doctorPrintf(out, "FAIL go: go version is unavailable or invalid\n"); err != nil {
				return err
			}
			failed = true
		} else {
			if err := doctorPrintf(out, "ok go: %s", version); err != nil {
				return err
			}
		}
		wasmExec, err := wasmserve.DefaultWASMExecPath()
		if err != nil {
			if err := doctorPrintf(out, "FAIL wasm_exec.js: %v\n", err); err != nil {
				return err
			}
			failed = true
		} else {
			if err := doctorPrintf(out, "ok wasm_exec.js: %s\n", wasmExec); err != nil {
				return err
			}
		}

		pkg := ""
		if len(args) == 1 {
			pkg = args[0]
		} else if info, statErr := os.Stat("wasm"); statErr == nil && info.IsDir() {
			pkg = "./wasm"
		}
		if pkg == "" {
			if err := doctorPrintf(out, "skipped wasm build: pass a package or run from a project with ./wasm\n"); err != nil {
				return err
			}
		} else {
			dir, tempErr := os.MkdirTemp("", "frameforge-doctor-*")
			if tempErr != nil {
				if err := doctorPrintf(out, "FAIL wasm build: %v\n", tempErr); err != nil {
					return err
				}
				failed = true
			} else {
				defer func() { _ = os.RemoveAll(dir) }()
				build := exec.Command("go", "build", "-o", filepath.Join(dir, "app.wasm"), pkg)
				build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
				if output, buildErr := build.CombinedOutput(); buildErr != nil {
					if err := doctorPrintf(out, "FAIL wasm build: %s\n", output); err != nil {
						return err
					}
					failed = true
				} else {
					if err := doctorPrintf(out, "ok wasm build: %s\n", pkg); err != nil {
						return err
					}
				}
			}
		}
		if failed {
			return fmt.Errorf("doctor found problems")
		}
		return nil
	},
}

func doctorPrintf(out io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(out, format, args...)
	return err
}

func init() { rootCmd.AddCommand(doctorCmd) }

package ffcli

import (
	"fmt"
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
			fmt.Fprintln(out, "FAIL go: go version is unavailable or invalid")
			failed = true
		} else {
			fmt.Fprintf(out, "ok go: %s", version)
		}
		wasmExec, err := wasmserve.DefaultWASMExecPath()
		if err != nil {
			fmt.Fprintf(out, "FAIL wasm_exec.js: %v\n", err)
			failed = true
		} else {
			fmt.Fprintf(out, "ok wasm_exec.js: %s\n", wasmExec)
		}

		pkg := ""
		if len(args) == 1 {
			pkg = args[0]
		} else if info, statErr := os.Stat("wasm"); statErr == nil && info.IsDir() {
			pkg = "./wasm"
		}
		if pkg == "" {
			fmt.Fprintln(out, "skipped wasm build: pass a package or run from a project with ./wasm")
		} else {
			dir, tempErr := os.MkdirTemp("", "frameforge-doctor-*")
			if tempErr != nil {
				fmt.Fprintf(out, "FAIL wasm build: %v\n", tempErr)
				failed = true
			} else {
				defer os.RemoveAll(dir)
				build := exec.Command("go", "build", "-o", filepath.Join(dir, "app.wasm"), pkg)
				build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
				if output, buildErr := build.CombinedOutput(); buildErr != nil {
					fmt.Fprintf(out, "FAIL wasm build: %s\n", output)
					failed = true
				} else {
					fmt.Fprintf(out, "ok wasm build: %s\n", pkg)
				}
			}
		}
		if failed {
			return fmt.Errorf("doctor found problems")
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }

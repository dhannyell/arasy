package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNativePackageArgPrefersTheNativeDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if got := nativePackageArg(nil); got != "." {
		t.Fatalf("nativePackageArg with no ./native = %q, want .", got)
	}
	if err := os.Mkdir(filepath.Join(dir, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := nativePackageArg(nil); got != "./native" {
		t.Fatalf("nativePackageArg with ./native = %q, want ./native", got)
	}
	if got := nativePackageArg([]string{"./elsewhere"}); got != "./elsewhere" {
		t.Fatalf("nativePackageArg with an explicit package = %q, want ./elsewhere", got)
	}
}

func TestSplitRunArgsSeparatesTheGamesArguments(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		argsLenAtDash int
		wantPkg       []string
		wantGame      []string
	}{
		{name: "no dash", args: []string{"./native"}, argsLenAtDash: -1, wantPkg: []string{"./native"}},
		{name: "dash only", args: []string{"--fullscreen"}, argsLenAtDash: 0, wantGame: []string{"--fullscreen"}},
		{name: "both", args: []string{"./native", "--level", "3"}, argsLenAtDash: 1, wantPkg: []string{"./native"}, wantGame: []string{"--level", "3"}},
	}
	for _, tc := range cases {
		pkg, game := splitRunArgs(tc.args, tc.argsLenAtDash)
		if strings.Join(pkg, " ") != strings.Join(tc.wantPkg, " ") {
			t.Fatalf("%s: package args = %v, want %v", tc.name, pkg, tc.wantPkg)
		}
		if strings.Join(game, " ") != strings.Join(tc.wantGame, " ") {
			t.Fatalf("%s: game args = %v, want %v", tc.name, game, tc.wantGame)
		}
	}
}

// The machinery, not the host: a throwaway binary stands in for a game so this
// runs on any machine, with or without a C toolchain or a display.
func TestRunGameBinaryForwardsOutputAndExitStatus(t *testing.T) {
	module := writeThrowawayModule(t, "game")
	out := t.TempDir()
	t.Chdir(module)
	if err := buildNative("./native", out, nativeBuildOptions{Mode: nativeModePortable}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(out, nativeBinaryName("./native"))

	var stdout, stderr bytes.Buffer
	err := runGameBinary(binary, []string{"fail"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("a game that exited non-zero reported success")
	}
	var coder exitCoder
	if !errors.As(err, &coder) {
		t.Fatalf("error %v does not carry an exit status", err)
	}
	if coder.ExitCode() != 3 {
		t.Fatalf("exit status = %d, want 3", coder.ExitCode())
	}
	if !strings.Contains(stdout.String(), "args: fail") {
		t.Fatalf("stdout = %q, want the game's own output including its arguments", stdout.String())
	}

	stdout.Reset()
	if err := runGameBinary(binary, nil, nil, &stdout, &stderr); err != nil {
		t.Fatalf("a game that exited zero reported %v", err)
	}
}

func TestPromptRebuildEndsCleanlyOnEOF(t *testing.T) {
	// A closed stdin is what a script or a CI job gives us; the loop has to
	// end there rather than block on a key that will never arrive.
	again, err := promptRebuild(bufio.NewReader(strings.NewReader("")), io.Discard)
	if err != nil {
		t.Fatalf("EOF reported %v, want a clean end", err)
	}
	if again {
		t.Fatal("EOF asked for another round")
	}

	in := bufio.NewReader(strings.NewReader("\n"))
	if again, err := promptRebuild(in, io.Discard); err != nil || !again {
		t.Fatalf("a newline gave (%v, %v), want another round", again, err)
	}
	if again, err := promptRebuild(in, io.Discard); err != nil || again {
		t.Fatalf("the input ran out and gave (%v, %v), want a clean end", again, err)
	}
}

// The whole loop, minus the window: build, run, prompt, and end at EOF. A
// leaked goroutine or a blocking read here would hang CI rather than fail it.
func TestRunDevNativeEndsAtEOF(t *testing.T) {
	module := writeThrowawayModule(t, "game")
	t.Chdir(module)

	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := runDevNative(cmd, "./native", nativeBuildOptions{Mode: nativeModePortable}); err != nil {
		t.Fatalf("runDevNative = %v, want a clean end at EOF", err)
	}
	if !strings.Contains(out.String(), "args:") {
		t.Fatalf("stdout = %q, want the game's own output", out.String())
	}
	if !strings.Contains(out.String(), "press Enter to rebuild") {
		t.Fatalf("stdout = %q, want the rebuild prompt", out.String())
	}
}

// One newline is one more round: the game runs twice and the loop still ends
// when the input runs out.
func TestRunDevNativeRebuildsOnEnter(t *testing.T) {
	module := writeThrowawayModule(t, "game")
	t.Chdir(module)

	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	if err := runDevNative(cmd, "./native", nativeBuildOptions{Mode: nativeModePortable}); err != nil {
		t.Fatalf("runDevNative = %v, want a clean end", err)
	}
	if runs := strings.Count(out.String(), "args:"); runs != 2 {
		t.Fatalf("the game ran %d times, want 2", runs)
	}
}

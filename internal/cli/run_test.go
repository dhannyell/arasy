package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

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

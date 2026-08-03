package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A default build of the CLI must not reach the native host. The cgo term on
// the watch --live seam is not enough on its own: cgo is on by default
// wherever a C compiler exists, which on Linux is nearly everywhere, and the
// host then drags in GLFW and ALSA headers that a machine has no reason to
// carry to run `arasy build`. Gating the seam on arr_dev as well is what
// keeps `go install` working on a plain machine.
func TestCLIDoesNotDependOnTheNativeHost(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./cmd/arasy")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "arasy/platform/native") {
		t.Fatal("the default CLI build imports platform/native; --live must stay behind arr_dev")
	}
}

// The other half of the same promise: even with the tag, a machine without a
// C toolchain has to be able to build the CLI.
//
// This is a build, not a link: it fails on the import graph, so it costs the
// same whether or not the host would have compiled.
func TestCLIBuildsWithoutCGO(t *testing.T) {
	output := filepath.Join(t.TempDir(), "arasy")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", output, "./cmd/arasy")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the CLI does not build without cgo: %v\n%s", err, out)
	}
}

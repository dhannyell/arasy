package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The CLI is pure Go and has to stay that way: a consumer installs it with
// `go install`, and a Windows machine without a C toolchain is exactly the
// machine that most needs it to work. Only the watch --live seam touches the
// native host, and its build tag carries the cgo term for this reason.
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

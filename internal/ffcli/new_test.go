package ffcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRenderProjectBuildsWithLocalEngine(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-game")
	err := renderProject(dir, projectTemplateData{
		Module: "example.com/my-game", Name: "mygame", GoVersion: strings.TrimPrefix(runtime.Version(), "go"),
		EngineModule: "github.com/dhannyell/frameforge", EngineVersion: "v0.0.0", EngineReplace: repoRoot(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := []struct {
		args []string
		env  []string
	}{
		{args: []string{"mod", "tidy"}},
		{args: []string{"build", "./..."}},
		{args: []string{"build", "-o", "output/app.wasm", "./wasm"}, env: []string{"GOOS=js", "GOARCH=wasm"}},
	}
	for _, command := range commands {
		cmd := exec.Command("go", command.args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
		cmd.Env = append(cmd.Env, command.env...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(command.args, " "), err, output)
		}
	}
}

func TestPackageNameSanitizesModulePath(t *testing.T) {
	if got := packageName("my-game"); got != "mygame" {
		t.Fatalf("packageName = %q, want mygame", got)
	}
}

func TestRenderProjectRefusesNonEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureDestinationEmpty(dir); err == nil {
		t.Fatal("ensureDestinationEmpty accepted non-empty directory")
	}
}

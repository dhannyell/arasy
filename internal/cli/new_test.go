package cli

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
		EngineModule: "github.com/dhannyell/arasy", EngineVersion: "v0.0.0", EngineReplace: repoRoot(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := []struct {
		args    []string
		env     []string
		network bool
	}{
		// tidy resolves a graph the engine's own build never has to load: the
		// native entry point reaches the audio decoders, and their go.mod
		// files are not in a cache warmed by building the engine alone. It is
		// also the one step a real user runs online, so let it reach the
		// proxy. The builds that follow stay offline, where an unexpected
		// download would mean the generated go.mod is incomplete.
		{args: []string{"mod", "tidy"}, network: true},
		{args: []string{"build", "./..."}},
		{args: []string{"build", "-o", "output/app.wasm", "./wasm"}, env: []string{"GOOS=js", "GOARCH=wasm"}},
	}
	for _, command := range commands {
		cmd := exec.Command("go", command.args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
		if !command.network {
			cmd.Env = append(cmd.Env, "GOPROXY=off")
		}
		cmd.Env = append(cmd.Env, command.env...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(command.args, " "), err, output)
		}
	}
}

func TestPackageNameSanitizesModulePath(t *testing.T) {
	for in, want := range map[string]string{
		"my-game": "mygame",
		// A Go identifier cannot start with a digit, and a module named after
		// a number is not hypothetical — examples/2048 is one.
		"2048":   "game2048",
		"UI Kit": "uikit",
		"---":    "",
	} {
		if got := packageName(in); got != want {
			t.Fatalf("packageName(%q) = %q, want %q", in, got, want)
		}
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

// The generated native entry point must carry the cgo term: a machine without
// a C toolchain has to keep building the rest of the project rather than fail
// on a binding it cannot compile.
func TestRenderProjectWritesANativeEntryPoint(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-game")
	err := renderProject(dir, projectTemplateData{
		Module: "example.com/my-game", Name: "mygame", GoVersion: strings.TrimPrefix(runtime.Version(), "go"),
		EngineModule: "github.com/dhannyell/arasy", EngineVersion: "v0.0.0", EngineReplace: repoRoot(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "native", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	const wantTag = "//go:build cgo && (windows || linux || darwin) && (amd64 || arm64)"
	if !strings.Contains(string(body), wantTag) {
		t.Fatalf("native/main.go does not carry %q:\n%s", wantTag, body)
	}
	if !strings.Contains(string(body), "native.NewLogger") {
		t.Fatalf("native/main.go does not set up a log file:\n%s", body)
	}
}

// A generated project repeats the engine's WebGPU replace because Go ignores
// one that comes from a dependency. That copy is only correct while it agrees
// with the original, so read the engine's go.mod back and compare.
func TestGeneratedGoModTracksTheEngineReplace(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	want := ""
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "replace "+webgpuModule+" => "); ok {
			want = strings.TrimSpace(after)
		}
	}
	if want == "" {
		t.Fatalf("the engine no longer replaces %s; drop the copy in new.go and the template line with it", webgpuModule)
	}
	if webgpuReplace != want {
		t.Fatalf("generated projects replace %s with %q, but the engine uses %q", webgpuModule, webgpuReplace, want)
	}
}

// A generated go.mod must require a version the module proxy can resolve.
// Only a released CLI knows one; a local build must name a replace instead.
func TestEngineVersionRequiresAResolvableVersion(t *testing.T) {
	cases := []struct {
		mainVersion, replace, want string
		wantErr                    bool
	}{
		{mainVersion: "v0.1.0", want: "v0.1.0"},
		{mainVersion: "v0.1.1-0.20261027120000-abcdef123456", want: "v0.1.1-0.20261027120000-abcdef123456"},
		{mainVersion: "(devel)", wantErr: true},
		{mainVersion: "v0.1.1-0.20261027120000-abcdef123456+dirty", wantErr: true},
		{mainVersion: "(devel)", replace: "../arasy", want: "v0.0.0"},
	}
	for _, c := range cases {
		got, err := engineVersion(c.mainVersion, c.replace)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("engineVersion(%q, %q) = %q, %v; want %q, error %v", c.mainVersion, c.replace, got, err, c.want, c.wantErr)
		}
	}
}

package ffcli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildWASMProducesStaticArtifacts(t *testing.T) {
	out := filepath.Join(t.TempDir(), "build")
	pkg := filepath.Join(repoRoot(t), "examples", "breakout", "wasm")
	if err := buildWASM(pkg, out, "Breakout", true, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app.wasm", "index.html", "wasm_exec.js"} {
		info, err := os.Stat(filepath.Join(out, name))
		if err != nil || info.Size() == 0 {
			t.Fatalf("artifact %s missing or empty: %v", name, err)
		}
	}
}

func TestSupportedPlatformRejectsUnknownPlatform(t *testing.T) {
	if err := supportedPlatform("native"); err == nil {
		t.Fatal("supportedPlatform(native) succeeded")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

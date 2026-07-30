package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/internal/wasmserve"
)

func TestWasmBuildArgsNoDefaultFont(t *testing.T) {
	withFlag := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, true), " ")
	if !strings.Contains(withFlag, "-tags arr_no_default_font") {
		t.Fatalf("args = %q, want -tags arr_no_default_font", withFlag)
	}
	withoutFlag := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, false), " ")
	if strings.Contains(withoutFlag, "-tags") {
		t.Fatalf("args = %q, want no -tags when the flag is off", withoutFlag)
	}
}

func TestBuildWASMProducesStaticArtifacts(t *testing.T) {
	out := filepath.Join(t.TempDir(), "build")
	pkg := filepath.Join(repoRoot(t), "examples", "breakout", "wasm")
	if err := buildWASM(pkg, out, "Breakout", true, false, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"app.wasm", "index.html", "wasm_exec.js",
		wasmserve.PosterLandscapeFile, wasmserve.PosterStackedFile,
	} {
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

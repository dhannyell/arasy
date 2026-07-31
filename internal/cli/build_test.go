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
	withFlag := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, true, false), " ")
	if !strings.Contains(withFlag, "-tags arr_no_default_font") {
		t.Fatalf("args = %q, want -tags arr_no_default_font", withFlag)
	}
	withoutFlag := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, false, false), " ")
	if strings.Contains(withoutFlag, "-tags") {
		t.Fatalf("args = %q, want no -tags when the flag is off", withoutFlag)
	}
}

func TestWasmBuildArgsDevTools(t *testing.T) {
	dev := strings.Join(wasmBuildArgs("pkg/wasm", "dist", false, false, true), " ")
	if !strings.Contains(dev, "-tags arr_dev") {
		t.Fatalf("dev args = %q, want -tags arr_dev", dev)
	}
	combined := strings.Join(wasmBuildArgs("pkg/wasm", "dist", false, true, true), " ")
	if !strings.Contains(combined, "-tags arr_no_default_font,arr_dev") {
		t.Fatalf("combined args = %q, want both tags in one -tags flag", combined)
	}
	release := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, false, false), " ")
	if strings.Contains(release, "arr_dev") {
		t.Fatalf("release args = %q, must never carry the dev panel tag", release)
	}
}

func TestBuildWASMProducesStaticArtifacts(t *testing.T) {
	out := filepath.Join(t.TempDir(), "build")
	pkg := filepath.Join(repoRoot(t), "examples", "breakout", "wasm")
	if err := buildWASM(pkg, out, "Breakout", wasmBuildOptions{Release: true}, io.Discard, io.Discard); err != nil {
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
	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(index)
	if !strings.Contains(html, "engine: true") || !strings.Contains(html, "game: true") {
		t.Fatal("static build does not use the default web splash policy")
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

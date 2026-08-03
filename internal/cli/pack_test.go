package cli

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/arasy/internal/wasmserve"
)

func TestPackContainsStaticBuildAtArchiveRoot(t *testing.T) {
	staging := t.TempDir()
	pkg := filepath.Join(repoRoot(t), "examples", "breakout", "wasm")
	if err := buildWASM(pkg, staging, "Breakout", wasmBuildOptions{Release: true}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "breakout.zip")
	if err := zipBuild(archive, staging); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	for _, name := range []string{
		"index.html", "app.wasm", "wasm_exec.js",
		wasmserve.PosterLandscapeFile, wasmserve.PosterStackedFile,
	} {
		if !found[name] {
			t.Fatalf("archive missing %s", name)
		}
	}
}

func TestPackNativeArchiveHoldsTheDepotLayout(t *testing.T) {
	module := writeThrowawayModule(t, "game")
	assets := filepath.Join(t.TempDir(), "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "manifest.json"), []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "game.zip")

	staging := t.TempDir()
	t.Chdir(module)
	if err := buildNative("./native", staging, nativeBuildOptions{
		Release: true, Mode: nativeModeBundle, AssetsDir: assets,
	}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := zipDir(archive, staging); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	for _, name := range []string{nativeBinaryName("./native"), "assets/manifest.json"} {
		if !found[name] {
			t.Fatalf("archive missing %s, has %v", name, found)
		}
	}
}

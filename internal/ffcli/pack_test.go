package ffcli

import (
	"archive/zip"
	"io"
	"path/filepath"
	"testing"
)

func TestPackContainsStaticBuildAtArchiveRoot(t *testing.T) {
	staging := t.TempDir()
	pkg := filepath.Join(repoRoot(t), "examples", "breakout", "wasm")
	if err := buildWASM(pkg, staging, "Breakout", true, io.Discard, io.Discard); err != nil {
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
	for _, name := range []string{"index.html", "app.wasm", "wasm_exec.js"} {
		if !found[name] {
			t.Fatalf("archive missing %s", name)
		}
	}
}

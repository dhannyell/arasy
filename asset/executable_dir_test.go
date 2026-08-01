// Unlike the other tests in this package, this one lives in package asset:
// it drives the executable-relative resolution through a fake executable
// path, which the exported entry point deliberately does not accept.
package asset

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestExecutableDirFSReportsMissingDirectory(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "game.exe")
	writeFile(t, exe, "binary")

	external, ok := dirFSNextTo(exe, "assets")
	if ok || external != nil {
		t.Fatalf("dirFSNextTo(missing) = (%v, %v), want (nil, false)", external, ok)
	}

	// A portable build loads exactly what it embedded.
	embedded := fstest.MapFS{
		"manifest.json":      {Data: []byte(`{"version":1,"audio":[{"id":"theme","file":"audio/theme.wav"}]}`)},
		"audio/theme.wav":    {Data: []byte("embedded-bytes")},
		"unrelated/file.txt": {Data: []byte("x")},
	}
	manifest, err := Load(embedded)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, err := fs.ReadFile(manifest.FS(), "audio/theme.wav")
	if err != nil {
		t.Fatalf("ReadFile(theme.wav) error = %v", err)
	}
	if string(got) != "embedded-bytes" {
		t.Fatalf("theme.wav bytes = %q, want embedded-bytes", got)
	}
}

func TestExecutableDirFSOverridesEmbeddedAsset(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "game.exe")
	writeFile(t, exe, "binary")
	writeFile(t, filepath.Join(dir, "assets", "audio", "theme.wav"), "external-bytes")

	external, ok := dirFSNextTo(exe, "assets")
	if !ok {
		t.Fatal("dirFSNextTo(present) reported no directory")
	}

	embedded := fstest.MapFS{
		"manifest.json":   {Data: []byte(`{"version":1,"audio":[{"id":"theme","file":"audio/theme.wav"}]}`)},
		"audio/theme.wav": {Data: []byte("embedded-bytes")},
	}
	manifest, err := Load(embedded, external)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, err := fs.ReadFile(manifest.FS(), "audio/theme.wav")
	if err != nil {
		t.Fatalf("ReadFile(theme.wav) error = %v", err)
	}
	if string(got) != "external-bytes" {
		t.Fatalf("theme.wav bytes = %q, want external-bytes", got)
	}
}

func TestExecutableDirFSResolvesThroughSymlink(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "install")
	exe := filepath.Join(install, "game.exe")
	writeFile(t, exe, "binary")
	writeFile(t, filepath.Join(install, "assets", "audio", "theme.wav"), "external-bytes")

	launch := filepath.Join(root, "launch")
	if err := os.MkdirAll(launch, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(launch, "game.exe")
	if err := os.Symlink(exe, link); err != nil {
		// Creating symlinks needs a privilege or developer mode on Windows.
		t.Skipf("symlinks unavailable: %v", err)
	}

	external, ok := dirFSNextTo(link, "assets")
	if !ok {
		t.Fatal("dirFSNextTo(symlink) reported no directory")
	}
	got, err := fs.ReadFile(external, "audio/theme.wav")
	if err != nil {
		t.Fatalf("ReadFile(theme.wav) error = %v", err)
	}
	if string(got) != "external-bytes" {
		t.Fatalf("theme.wav bytes = %q, want external-bytes", got)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

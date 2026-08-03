// Unlike the other tests in this package, this one lives in package asset:
// it drives the executable-relative resolution through a fake executable
// path, which the exported entry point deliberately does not accept.
//
// Layering itself is not retested here — manifest_fs_test.go owns that. What
// is on trial is where the layer comes from.
package asset

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableDirFSReportsMissingDirectory(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "game.exe")
	writeFile(t, exe, "binary")

	external, ok := dirFSNextTo(exe, "assets")
	if ok || external != nil {
		t.Fatalf("dirFSNextTo(missing) = (%v, %v), want (nil, false)", external, ok)
	}
}

func TestExecutableDirFSReadsTheDirectoryBesideTheExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "game.exe")
	writeFile(t, exe, "binary")
	writeFile(t, filepath.Join(dir, "assets", "audio", "theme.wav"), "external-bytes")

	// The working directory is deliberately somewhere else: resolving from it
	// instead of from the executable is the bug this function exists to avoid.
	t.Chdir(t.TempDir())

	external, ok := dirFSNextTo(exe, "assets")
	if !ok {
		t.Fatal("dirFSNextTo(present) reported no directory")
	}
	assertReads(t, external, "audio/theme.wav", "external-bytes")
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
	assertReads(t, external, "audio/theme.wav", "external-bytes")
}

func assertReads(t *testing.T, fsys fs.FS, name, want string) {
	t.Helper()
	got, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", name, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
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

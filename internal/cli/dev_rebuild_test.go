package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFingerprintSourcesDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main")

	first, err := fingerprintSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	again, err := fingerprintSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatal("fingerprint changed without any file change")
	}

	write("main.go", "package main // edited")
	edited, err := fingerprintSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if edited == first {
		t.Fatal("fingerprint unchanged after editing a file")
	}

	write("asset.png", "fake-png-bytes")
	added, err := fingerprintSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added == edited {
		t.Fatal("fingerprint unchanged after adding an asset file")
	}
}

func TestFingerprintSourcesSkipsBuildDirectories(t *testing.T) {
	dir := t.TempDir()
	noise := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(noise, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := fingerprintSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	// mtime resolution differs across filesystems; ensure a visible bump.
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(noise, "junk.js"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := fingerprintSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("fingerprint changed when only a skipped directory changed")
	}
}

func TestSameFileContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.wasm")
	b := filepath.Join(dir, "b.wasm")
	if err := os.WriteFile(a, []byte("identical"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("identical"), 0o644); err != nil {
		t.Fatal(err)
	}

	same, err := sameFileContent(a, b)
	if err != nil || !same {
		t.Fatalf("sameFileContent(identical) = %v, %v; want true, nil", same, err)
	}

	if err := os.WriteFile(b, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	same, err = sameFileContent(a, b)
	if err != nil || same {
		t.Fatalf("sameFileContent(different) = %v, %v; want false, nil", same, err)
	}

	same, err = sameFileContent(a, filepath.Join(dir, "missing.wasm"))
	if err != nil || same {
		t.Fatalf("sameFileContent(missing) = %v, %v; want false, nil", same, err)
	}
}

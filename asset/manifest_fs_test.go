package asset_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
)

func TestManifestFSReadsFromLastLayerThatHasThePath(t *testing.T) {
	base := fstest.MapFS{
		"manifest.json":   {Data: []byte(`{"version":1,"audio":[{"id":"theme","file":"audio/theme.wav"}]}`)},
		"audio/theme.wav": {Data: []byte("base-bytes")},
	}
	override := fstest.MapFS{
		"audio/theme.wav": {Data: []byte("override-bytes")},
	}

	m, err := asset.Load(base, override)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, err := fs.ReadFile(m.FS(), "audio/theme.wav")
	if err != nil {
		t.Fatalf("ReadFile(theme.wav) error = %v", err)
	}
	if string(got) != "override-bytes" {
		t.Fatalf("theme.wav bytes = %q, want override-bytes", got)
	}
}

func TestManifestFSFallsBackToEarlierLayerWhenLaterLayerLacksThePath(t *testing.T) {
	base := fstest.MapFS{
		"manifest.json":   {Data: []byte(`{"version":1,"audio":[{"id":"theme","file":"audio/theme.wav"}]}`)},
		"audio/theme.wav": {Data: []byte("base-bytes")},
	}
	override := fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1}`)},
	}

	m, err := asset.Load(base, override)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, err := fs.ReadFile(m.FS(), "audio/theme.wav")
	if err != nil {
		t.Fatalf("ReadFile(theme.wav) error = %v", err)
	}
	if string(got) != "base-bytes" {
		t.Fatalf("theme.wav bytes = %q, want base-bytes", got)
	}
}

func TestManifestFSReturnsNotExistForUnknownPath(t *testing.T) {
	m, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1}`)},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	_, err = m.FS().Open("nope.wav")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open(nope.wav) error = %v, want fs.ErrNotExist", err)
	}
}

func TestManifestZeroValueFSIsSafeAndReportsNotExist(t *testing.T) {
	var m asset.Manifest

	_, err := m.FS().Open("anything")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open(anything) error = %v, want fs.ErrNotExist", err)
	}
}

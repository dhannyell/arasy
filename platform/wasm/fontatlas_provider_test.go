package wasm

import (
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"golang.org/x/image/font/gofont/goregular"
)

func TestFontAtlasProviderRetainsAndGrowsAtlasAcrossRequests(t *testing.T) {
	p := testFontAtlasProvider(t)
	first, err := p.AtlasFor(1, 18, "Áê")
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.AtlasFor(1, 18, "êÁ")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("atlas changed without any newly requested rune")
	}
	for _, r := range []rune{'Á', 'ê'} {
		if _, ok := first.Glyphs[r]; !ok {
			t.Fatalf("prepared atlas is missing %q", r)
		}
	}
}

func TestFontAtlasProviderToleratesCorruptManifestFont(t *testing.T) {
	m, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"bad","file":"bad.ttf"}]}`)},
		"bad.ttf":       {Data: []byte("not a font")},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := newFontAtlasProvider(m)
	if err != nil {
		t.Fatalf("newFontAtlasProvider() error = %v, want the provider to degrade instead of failing", err)
	}
	if _, err := p.AtlasFor(m.Font("bad"), 18, "text"); err == nil {
		t.Fatal("AtlasFor() for the corrupt font succeeded, want an error so callers fall back to the built-in atlas")
	}
}

func testFontAtlasProvider(t *testing.T) *fontAtlasProvider {
	t.Helper()
	m, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"heading","file":"heading.ttf"}]}`)},
		"heading.ttf":   {Data: goregular.TTF},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := newFontAtlasProvider(m)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

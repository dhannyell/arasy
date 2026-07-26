package fontatlas_test

import (
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/render/gpu/fontatlas"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"golang.org/x/image/font/gofont/goregular"
)

func TestProviderRetainsAndGrowsAtlasAcrossRequests(t *testing.T) {
	p := testProvider(t)
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

func TestProviderToleratesCorruptManifestFont(t *testing.T) {
	m, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"bad","file":"bad.ttf"}]}`)},
		"bad.ttf":       {Data: []byte("not a font")},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := fontatlas.New(m)
	if err != nil {
		t.Fatalf("New() error = %v, want the provider to degrade instead of failing", err)
	}
	if _, err := p.AtlasFor(m.Font("bad"), 18, "text"); err == nil {
		t.Fatal("AtlasFor() for the corrupt font succeeded, want an error so callers fall back to the built-in atlas")
	}
}

func testProvider(t *testing.T) *fontatlas.Provider {
	t.Helper()
	m, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"heading","file":"heading.ttf"}]}`)},
		"heading.ttf":   {Data: goregular.TTF},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := fontatlas.New(m)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProviderDefaultFontAtlasGrowsWithText(t *testing.T) {
	if typeface.DefaultFace() == nil {
		t.Skip("default font compiled out (arr_no_default_font)")
	}
	p := testProvider(t)

	atlas, err := p.AtlasFor(0, 18, "ção")
	if err != nil {
		t.Fatal(err)
	}
	if atlas.Key != (tess.AtlasKey{Font: 0, PX: 18}) {
		t.Fatalf("atlas.Key = %+v, want {Font: 0, PX: 18}", atlas.Key)
	}
	for _, r := range []rune{'ç', 'ã', 'o'} {
		if _, ok := atlas.Glyphs[r]; !ok {
			t.Fatalf("default font atlas is missing %q", r)
		}
	}

	again, err := p.AtlasFor(0, 18, "ço")
	if err != nil {
		t.Fatal(err)
	}
	if again != atlas {
		t.Fatal("atlas rebuilt without any newly requested rune")
	}

	grown, err := p.AtlasFor(0, 18, "é")
	if err != nil {
		t.Fatal(err)
	}
	if grown == atlas {
		t.Fatal("atlas not rebuilt for a newly requested rune")
	}
	if _, ok := grown.Glyphs['é']; !ok {
		t.Fatal("grown atlas is missing 'é'")
	}

	cached, ok := p.CachedAtlas(0, 18)
	if !ok || cached != grown {
		t.Fatal("CachedAtlas(0, 18) did not return the grown default atlas")
	}
}

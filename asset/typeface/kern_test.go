package typeface_test

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// TestMeasureKerningOnRealFont documents the actual, observed kerning
// behavior of golang.org/x/image/font/gofont/goregular, rather than
// asserting behavior the font may not have.
//
// Go Regular, like many modern fonts, ships glyph-positioning data only in
// GPOS (an OpenType feature table golang.org/x/image/font/sfnt does not
// read); it has no legacy "kern" table. sfnt.Font.Kern therefore returns no
// adjustment for any glyph pair, so Face.Measure applies zero kerning: a
// pair's width always equals the sum of its glyphs' individual advances,
// even for classic kerning pairs like "AV" or "To".
func TestMeasureKerningOnRealFont(t *testing.T) {
	face := mustParse(t, goregular.TTF)

	for _, pair := range []struct{ left, right string }{
		{"A", "V"},
		{"T", "o"},
	} {
		sum := face.Measure(pair.left, 64).X + face.Measure(pair.right, 64).X
		combined := face.Measure(pair.left+pair.right, 64).X
		if combined != sum {
			t.Fatalf("Measure(%q) = %v, want the unkerned sum %v (Go Regular has no legacy kern table)",
				pair.left+pair.right, combined, sum)
		}
	}
}

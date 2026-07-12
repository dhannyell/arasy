package typeface

import (
	"errors"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type fakeMetricsFont struct {
	glyphIndexErr   error
	advanceErrGlyph sfnt.GlyphIndex
	kern            fixed.Int26_6
	kernErr         error
	metricsErr      error
}

func (f fakeMetricsFont) GlyphIndex(_ *sfnt.Buffer, r rune) (sfnt.GlyphIndex, error) {
	if f.glyphIndexErr != nil {
		return 0, f.glyphIndexErr
	}
	return sfnt.GlyphIndex(r - 'A' + 1), nil
}

func (f fakeMetricsFont) GlyphAdvance(_ *sfnt.Buffer, glyph sfnt.GlyphIndex, _ fixed.Int26_6, hinting font.Hinting) (fixed.Int26_6, error) {
	if hinting != font.HintingNone {
		panic("measure used font hinting")
	}
	if glyph == f.advanceErrGlyph && glyph != 0 {
		return 0, errors.New("advance failed")
	}
	if glyph == 0 {
		return fixed.I(5), nil
	}
	return fixed.I(int(glyph) * 10), nil
}

func (f fakeMetricsFont) Kern(_ *sfnt.Buffer, _, _ sfnt.GlyphIndex, _ fixed.Int26_6, hinting font.Hinting) (fixed.Int26_6, error) {
	if hinting != font.HintingNone {
		panic("measure used font hinting")
	}
	return f.kern, f.kernErr
}

func (f fakeMetricsFont) Metrics(_ *sfnt.Buffer, _ fixed.Int26_6, hinting font.Hinting) (font.Metrics, error) {
	if hinting != font.HintingNone {
		panic("measure used font hinting")
	}
	return font.Metrics{Height: fixed.I(40)}, f.metricsErr
}

func TestMeasureAddsSuccessfulKerning(t *testing.T) {
	got := measure(fakeMetricsFont{kern: fixed.I(-2)}, &sfnt.Buffer{}, "AB", 16)
	if got.X != 28 || got.Y != 40 {
		t.Fatalf("measure() = %v, want width 10 + 20 - 2 and height 40", got)
	}
}

func TestMeasureDegradesGlyphErrorsAndIgnoresKerningErrors(t *testing.T) {
	t.Run("glyph index uses NotDef", func(t *testing.T) {
		got := measure(fakeMetricsFont{glyphIndexErr: errors.New("index failed")}, &sfnt.Buffer{}, "A", 16)
		if got.X != 5 {
			t.Fatalf("measure() width = %v, want NotDef advance 5", got.X)
		}
	})

	t.Run("glyph advance uses NotDef", func(t *testing.T) {
		got := measure(fakeMetricsFont{advanceErrGlyph: 1}, &sfnt.Buffer{}, "A", 16)
		if got.X != 5 {
			t.Fatalf("measure() width = %v, want NotDef advance 5", got.X)
		}
	})

	t.Run("kerning error is ignored", func(t *testing.T) {
		got := measure(fakeMetricsFont{kern: fixed.I(-2), kernErr: errors.New("kern missing")}, &sfnt.Buffer{}, "AB", 16)
		if got.X != 30 {
			t.Fatalf("measure() width = %v, want unkerned width 30", got.X)
		}
	})

	t.Run("metrics error preserves width", func(t *testing.T) {
		got := measure(fakeMetricsFont{metricsErr: errors.New("metrics failed")}, &sfnt.Buffer{}, "A", 16)
		if got.X != 10 || got.Y != 0 {
			t.Fatalf("measure() = %v, want width 10 and zero height", got)
		}
	})
}

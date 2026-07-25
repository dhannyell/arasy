// Package typeface parses manifest-declared TTF/OTF fonts and computes
// text metrics in pure Go. Metrics are deterministic across platforms
// and architectures (fixed-point sfnt math, no OS font stack), so they
// are safe to use for layout on the simulation side of the engine.
package typeface

import (
	"errors"
	"fmt"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Face is a parsed font ready for measuring and rasterizing.
type Face struct {
	font *sfnt.Font
	// buf is reused across measurements; Face methods are NOT safe for
	// concurrent use (callers keep one Face per goroutine or lock)
	buf sfnt.Buffer
}

type metricsFont interface {
	GlyphIndex(*sfnt.Buffer, rune) (sfnt.GlyphIndex, error)
	GlyphAdvance(*sfnt.Buffer, sfnt.GlyphIndex, fixed.Int26_6, font.Hinting) (fixed.Int26_6, error)
	Kern(*sfnt.Buffer, sfnt.GlyphIndex, sfnt.GlyphIndex, fixed.Int26_6, font.Hinting) (fixed.Int26_6, error)
	Metrics(*sfnt.Buffer, fixed.Int26_6, font.Hinting) (font.Metrics, error)
}

type Registry []*Face

// BuildRegistry decodes every font in the manifest. A font that fails
// to decode is left nil in the registry (the same "missing font" state
// Lookup already reports for an unknown id) instead of aborting the whole
// registry, so one corrupt asset never blocks the others; every decode error
// is collected and returned joined.
func BuildRegistry(m asset.Manifest) (Registry, error) {
	decls := m.Fonts()
	reg := make(Registry, len(decls)+1)
	var errs []error
	for _, d := range decls {
		fontFace, err := Parse(d.Data)
		if err != nil {
			errs = append(errs, fmt.Errorf("decode font %q: %w", d.Name, err))
			continue
		}
		reg[d.ID] = fontFace
	}
	return reg, errors.Join(errs...)
}

// Lookup returns the parsed Face for id, or nil when id is 0 or unknown.
func (r Registry) Lookup(id render.FontID) *Face {
	if id == 0 || int(id) >= len(r) {
		return nil
	}
	return r[id]
}

func Parse(data []byte) (*Face, error) {
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return &Face{font: f}, nil
}

func (f *Face) Font() *sfnt.Font {
	return f.font
}

// Measure returns the advance width and line height of text at the given
// size (world units == pixels at scale 1). Kerning from the font's kern
// table is applied between consecutive glyphs. Unknown runes fall back to
// the font's NotDef glyph advance.
func (f *Face) Measure(text string, size float64) math2d.Vec2 {
	return measure(f.font, &f.buf, text, size)
}

func measure(fontMetrics metricsFont, buf *sfnt.Buffer, text string, size float64) math2d.Vec2 {
	if size <= 0 {
		return math2d.Vec2{}
	}

	ppem := fixed.Int26_6(size * 64)
	var width fixed.Int26_6
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, r := range text {
		g, err := fontMetrics.GlyphIndex(buf, r)
		if err != nil || g == 0 {
			g = 0
		}

		adv, err := fontMetrics.GlyphAdvance(buf, g, ppem, font.HintingNone)
		if err != nil && g != 0 {
			g = 0
			adv, _ = fontMetrics.GlyphAdvance(buf, g, ppem, font.HintingNone)
		}
		width += adv

		if hasPrev {
			k, err := fontMetrics.Kern(buf, prev, g, ppem, font.HintingNone)
			if err == nil {
				width += k
			}
		}

		prev = g
		hasPrev = true
	}

	met, err := fontMetrics.Metrics(buf, ppem, font.HintingNone)
	if err != nil {
		return math2d.Vec2{X: fromFixed(width)}
	}

	return math2d.Vec2{
		X: fromFixed(width),
		Y: fromFixed(met.Height),
	}
}

// Ascent returns the ascent at size, for baseline placement.
func (f *Face) Ascent(size float64) float64 {
	if size <= 0 {
		return 0
	}

	ppem := fixed.Int26_6(size * 64)
	met, err := f.font.Metrics(&f.buf, ppem, font.HintingNone)
	if err != nil {
		return 0
	}
	return fromFixed(met.Ascent)
}

func fromFixed(f fixed.Int26_6) float64 {
	return float64(f) / 64.0
}

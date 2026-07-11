// Package typeface parses manifest-declared TTF/OTF fonts and computes
// text metrics in pure Go. Metrics are deterministic across platforms
// and architectures (fixed-point sfnt math, no OS font stack), so they
// are safe to use for layout on the simulation side of the engine.
package typeface

import (
	"errors"
	"fmt"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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

// Lookup returns the pixels for id, or nil when the id is unknown.
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
	ppem := fixed.Int26_6(size * 64)
	var width fixed.Int26_6
	var prev sfnt.GlyphIndex
	hasPrev := false
	for _, r := range text {
		g, err := f.font.GlyphIndex(&f.buf, r)
		if err != nil || g == 0 {
			g = 0
		}

		adv, _ := f.font.GlyphAdvance(&f.buf, g, ppem, font.HintingNone)
		width += adv

		if hasPrev {
			k, err := f.font.Kern(&f.buf, prev, g, ppem, font.HintingNone)
			if err != nil {
				width += k
			}
		}

		prev = g
		hasPrev = true
	}

	met, _ := f.font.Metrics(&f.buf, ppem, font.HintingNone)
	lineHeight := met.Ascent - met.Descent + met.CapHeight

	return math2d.Vec2{
		X: fromFixed(width),
		Y: fromFixed(lineHeight),
	}
}

// Ascent returns the ascent at size, for baseline placement.
func (f *Face) Ascent(size float64) float64 {
	ppem := fixed.Int26_6(size * 64)
	met, _ := f.font.Metrics(&f.buf, ppem, font.HintingNone)
	return fromFixed(met.Ascent)
}

func fromFixed(f fixed.Int26_6) float64 {
	return float64(f) / 64.0
}

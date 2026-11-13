// Package slugcore is the part of Slug text that the hosts link. It has no text
// shaper, so a game that does not set GlyphOutlines does not pay for one.
package slugcore

import (
	"errors"

	slug "github.com/dhannyell/go-slug"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/render"
)

// ErrNoFont reports a font that an Engine cannot use. The caller draws the
// command with the glyph atlas.
var ErrNoFont = errors.New("slugtext: font is not available")

// Glyph is one glyph that is ready to draw.
type Glyph struct {
	Entry slug.Entry
	View  slug.Affine // from em space to screen pixels, Y down
	Color [4]float32  // straight RGB and alpha
}

// Engine draws DrawText commands from glyph outlines.
type Engine interface {
	tess.SlugText
	// Pack returns the Pack that holds the entries of the glyphs.
	Pack() *slug.Pack
	// Append appends the glyphs of cmd to dst.
	Append(dst []Glyph, cmd render.DrawText, cam render.Camera) ([]Glyph, error)
	// EndFrame ends a frame.
	EndFrame()
	// TakeErr returns the first error of AppendInstances since the last call.
	TakeErr() error
}

// Factory returns an Engine. lookup finds the font of an ID other than 0; ID 0 is
// the default font of the engine.
type Factory func(lookup func(render.FontID) *typeface.Face) Engine

// WGSL returns the complete shader of the records of AppendInstances. Its entry
// points are vs_main and fs_main.
func WGSL() string { return slug.CoreWGSL + shaders.SlugTextWGSL }

// Package slug enables render.GlyphSlug text. A game imports it only if it draws
// that text, because the text shaper adds about 900 KB of gzip to a wasm binary.
package slug

import (
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/internal/render/slugtext"
	"github.com/dhannyell/arasy/render"
)

// Outlines enables render.GlyphSlug text. Set it in the GlyphOutlines field of a
// host, for example wasm.Options{GlyphOutlines: slug.Outlines}.
func Outlines(lookup func(render.FontID) *typeface.Face) slugcore.Engine {
	return slugtext.New(lookup)
}

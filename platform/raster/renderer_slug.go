package raster

import (
	"errors"
	"image"
	"log/slog"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/render"
)

// drawSlugText draws command from glyph outlines. It returns false if the caller
// must use the glyph atlas.
func (r *Renderer) drawSlugText(img *image.RGBA, command render.DrawText) bool {
	if r.slug == nil {
		if r.GlyphOutlines == nil {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "slug-text-off"},
				"raster: GlyphSlug text uses the glyph atlas; set GlyphOutlines to slug.Outlines")
			return false
		}
		r.slug = r.GlyphOutlines(func(id render.FontID) *typeface.Face { return r.Fonts.Lookup(id) })
	}
	glyphs, err := r.slug.Append(r.slugGlyphs[:0], command, r.Camera)
	r.slugGlyphs = glyphs
	if errors.Is(err, slugcore.ErrNoFont) {
		return false
	}
	if err != nil {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "slug-text-add"},
			"raster: Slug text is incomplete this frame", "error", err)
	}
	dst, ok := img.SubImage(r.clipBounds(img, img.Bounds(), command.Clip)).(*image.RGBA)
	if !ok {
		return true
	}
	pack := r.slug.Pack()
	for _, g := range glyphs {
		// The only error is a view without an inverse: it has no visible area.
		_ = pack.Rasterize(g.Entry, g.View, g.Color, dst)
	}
	return true
}

package fontatlas

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/internal/render/gpu/tess"
	"github.com/dhannyell/frameforge/render"
)

// Provider owns CPU-side glyph atlases. tess.Build submits all text for one
// (font, px) key in a single request, so a frame observes one stable atlas
// layout while the provider can retain and grow it across frames.
type Provider struct {
	builtin *tess.Atlas
	fonts   typeface.Registry
	atlases map[tess.AtlasKey]*tess.Atlas
	runes   map[tess.AtlasKey]map[rune]struct{}
}

// New builds a provider from whatever typeface.BuildRegistry manages to
// decode. BuildRegistry deliberately returns a usable partial registry
// alongside a joined error: a font that fails to decode is left nil (AtlasFor
// then reports it as unknown and callers fall back to the built-in font), so
// one corrupt manifest font degrades gracefully instead of aborting the GPU
// renderer. This mirrors how platform/raster/run.go treats the same error.
func New(manifest asset.Manifest) (*Provider, error) {
	fonts, err := typeface.BuildRegistry(manifest)
	if err != nil {
		slog.Default().Warn("partial font registry", "error", err)
	}
	return &Provider{
		builtin: tess.NewAtlas(),
		fonts:   fonts,
		atlases: make(map[tess.AtlasKey]*tess.Atlas),
		runes:   make(map[tess.AtlasKey]map[rune]struct{}),
	}, nil
}

// AtlasFor returns the requested atlas, growing it to cover text when needed.
func (p *Provider) AtlasFor(fontID render.FontID, px int, text string) (*tess.Atlas, error) {
	if fontID == 0 {
		return p.builtin, nil
	}
	return p.ensure(fontID, px, text)
}

// CachedAtlas returns a previously materialized atlas without growing it.
func (p *Provider) CachedAtlas(fontID render.FontID, px int) (*tess.Atlas, bool) {
	if fontID == 0 {
		return p.builtin, true
	}
	atlas, ok := p.atlases[tess.AtlasKey{Font: fontID, PX: px}]
	return atlas, ok
}

func (p *Provider) ensure(fontID render.FontID, px int, text string) (*tess.Atlas, error) {
	face := p.fonts.Lookup(fontID)
	if face == nil {
		return nil, errors.New("unknown font id")
	}
	key := tess.AtlasKey{Font: fontID, PX: px}
	set := p.runes[key]
	if set == nil {
		set = make(map[rune]struct{})
		p.runes[key] = set
	}
	changed := p.atlases[key] == nil
	for _, r := range text {
		if _, exists := set[r]; !exists {
			set[r] = struct{}{}
			changed = true
		}
	}
	if !changed {
		return p.atlases[key], nil
	}
	runes := make([]rune, 0, len(set))
	for r := range set {
		runes = append(runes, r)
	}
	atlas, err := tess.NewAtlasForFace(fontID, px, face, runes)
	if err != nil {
		return nil, fmt.Errorf("build font atlas %d at %dpx: %w", fontID, px, err)
	}
	p.atlases[key] = atlas
	return atlas, nil
}

var _ tess.CachedAtlasProvider = (*Provider)(nil)

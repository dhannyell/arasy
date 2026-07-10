package runtime

import (
	"image"
	"sort"

	"github.com/dhannyell/frameforge/render"
)

// TextureUpdate carries new pixel data for a dynamic texture.
// The Image ownership is transferred to the consumer - after calling
// UpdateTexture the caller must not mutate the *image.RGBA.
type TextureUpdate struct {
	ID    render.TextureID
	Image *image.RGBA
}

// driverTexture owns the producer side of texture updates.
// This is render-only state: never snapshotted, never recorded.
// Updates are keyed by ID so multiple UpdateTexture calls for the same
// texture in one frame collapse to the last one.
type driverTexture struct {
	pending map[render.TextureID]*image.RGBA
}

func (t *driverTexture) update(id render.TextureID, img *image.RGBA) {
	if t.pending == nil {
		t.pending = make(map[render.TextureID]*image.RGBA)
	}
	t.pending[id] = img
}

// consume returns the pending updates sorted by TextureID for deterministic
// ordering and zeroes the pending set.
func (t *driverTexture) consume() []TextureUpdate {
	if len(t.pending) == 0 {
		return nil
	}
	out := make([]TextureUpdate, 0, len(t.pending))
	for id, img := range t.pending {
		out = append(out, TextureUpdate{ID: id, Image: img})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	t.pending = nil
	return out
}

func (t *driverTexture) reset() {
	t.pending = nil
}

// TexturePort is the game-facing texture update API, reachable
// inside Game.Render via RenderContext.UpdateTexture.
type TexturePort struct {
	texture *driverTexture
}

func (p *TexturePort) UpdateTexture(id render.TextureID, img *image.RGBA) {
	p.texture.update(id, img)
}

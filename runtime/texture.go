package runtime

import (
	"image"

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
type driverTexture struct {
	pending []TextureUpdate
}

func (t *driverTexture) update(id render.TextureID, img *image.RGBA) {
	t.pending = append(t.pending, TextureUpdate{ID: id, Image: img})
}

func (t *driverTexture) consume() []TextureUpdate {
	out := t.pending
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

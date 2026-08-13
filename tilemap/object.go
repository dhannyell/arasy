package tilemap

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// TileObject is one editor tile object with its draw geometry already
// resolved: Dest in world units, Src in atlas pixels, rotation around the
// Dest center.
type TileObject struct {
	Texture  render.TextureID
	Dest     math2d.Rect
	Src      math2d.Rect
	Flip     render.SpriteFlip
	Rotation float64
}

// ObjectView publishes tile objects as retained batches, one batch per
// texture, so the scene the artist authored draws without game code.
type ObjectView struct {
	Objects []TileObject

	base      render.SpriteBatchID
	textures  []render.TextureID
	buffer    []render.SpriteInstance
	published bool
}

// NewObjectView pairs tile objects with a base batch ID. Each distinct
// texture takes one ID starting at base, in order of first appearance;
// Batches reports how many the view occupies.
func NewObjectView(objects []TileObject, base render.SpriteBatchID) *ObjectView {
	v := &ObjectView{Objects: objects, base: base}
	for _, object := range objects {
		if !containsTexture(v.textures, object.Texture) {
			v.textures = append(v.textures, object.Texture)
		}
	}
	return v
}

// Batches returns the number of batch IDs the view occupies from its base.
func (v *ObjectView) Batches() int {
	return len(v.textures)
}

// Publish uploads the objects to the sink once; later calls do nothing.
func (v *ObjectView) Publish(sink BatchSink) {
	if v.published {
		return
	}
	v.published = true
	for index, texture := range v.textures {
		v.buffer = v.buffer[:0]
		for _, object := range v.Objects {
			if object.Texture != texture {
				continue
			}
			v.buffer = append(v.buffer, render.SpriteInstance{
				Texture:        object.Texture,
				ProjectCorners: true,
				Dest:           object.Dest,
				Src:            object.Src,
				Flip:           object.Flip,
				Rotation:       object.Rotation,
			})
		}
		sink.UpdateSprites(v.base+render.SpriteBatchID(index), v.buffer)
	}
}

// Commands appends one DrawSpriteBatch per published texture to dst.
func (v *ObjectView) Commands(dst []render.Command, layer int) []render.Command {
	for index := range v.textures {
		dst = append(dst, render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Batch:       v.base + render.SpriteBatchID(index),
		})
	}
	return dst
}

func containsTexture(textures []render.TextureID, texture render.TextureID) bool {
	for _, existing := range textures {
		if existing == texture {
			return true
		}
	}
	return false
}

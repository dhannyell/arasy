package tilemap

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// TileObject is one editor tile object with its draw geometry already
// resolved: Dest in world units, Src in atlas pixels, rotation around the
// Dest center.
type TileObject struct {
	Texture   render.TextureID
	Dest      math2d.Rect
	Src       math2d.Rect
	Flip      render.SpriteFlip
	Rotation  float64
	Animation render.SpriteAnimation
}

// ObjectView publishes tile objects as one retained batch, preserving the
// artist's draw order across texture changes.
type ObjectView struct {
	Objects []TileObject

	base      render.SpriteBatchID
	buffer    []render.SpriteInstance
	published bool
}

// NewObjectView pairs tile objects with one batch ID at base.
func NewObjectView(objects []TileObject, base render.SpriteBatchID) *ObjectView {
	return &ObjectView{Objects: objects, base: base}
}

// Batches returns the number of batch IDs the view occupies from its base.
func (v *ObjectView) Batches() int {
	if len(v.Objects) == 0 {
		return 0
	}
	return 1
}

// Publish uploads the objects to the sink once; later calls do nothing.
func (v *ObjectView) Publish(sink BatchSink) {
	if v.published {
		return
	}
	v.published = true
	if len(v.Objects) == 0 {
		return
	}
	v.buffer = v.buffer[:0]
	for _, object := range v.Objects {
		v.buffer = append(v.buffer, render.SpriteInstance{
			Texture:        object.Texture,
			ProjectCorners: true,
			Dest:           object.Dest,
			Src:            object.Src,
			Flip:           object.Flip,
			Rotation:       object.Rotation,
			Animation:      object.Animation,
		})
	}
	sink.UpdateSprites(v.base, v.buffer)
}

// Commands appends the object's batch to dst. time drives tile flip-books.
func (v *ObjectView) Commands(dst []render.Command, layer int, time float64) []render.Command {
	if len(v.Objects) == 0 {
		return dst
	}
	return append(dst, render.DrawSpriteBatch{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Batch:       v.base,
		Animation:   render.SpriteBatchAnimation{Time: time},
	})
}

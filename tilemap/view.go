package tilemap

import "github.com/dhannyell/arasy/render"

// BatchSink receives retained sprite batches. *runtime.BatchPort satisfies it.
type BatchSink interface {
	UpdateSprites(id render.SpriteBatchID, instances []render.SpriteInstance)
}

// View publishes one layer as retained chunk batches and emits the draw
// commands for the visible ones. Mutate Layer through its Set methods and the
// next Publish uploads only the dirty chunks. Buffers are reused across calls,
// so the steady state allocates nothing.
type View struct {
	Layer Layer
	Atlas Atlas

	base      render.SpriteBatchID
	buffer    []render.SpriteInstance
	chunks    []ChunkCoord
	published bool
}

// NewView pairs a layer with its atlas. Each chunk uses one batch ID starting
// at base; reserve one ID per chunk before the next view's base.
func NewView(layer Layer, atlas Atlas, base render.SpriteBatchID) *View {
	return &View{Layer: layer, Atlas: atlas, base: base}
}

// Publish uploads the layer's chunks to the sink: every chunk on the first
// call, only the dirty ones after. Call it from Init and Update.
func (v *View) Publish(sink BatchSink) {
	if !v.published {
		v.published = true
		for y := 0; y < v.Layer.H; y += ChunkSize {
			for x := 0; x < v.Layer.W; x += ChunkSize {
				v.publishChunk(sink, ChunkCoord{X: x / ChunkSize, Y: y / ChunkSize})
			}
		}
		v.Layer.DirtyChunks()
		return
	}
	for _, chunk := range v.Layer.DirtyChunks() {
		v.publishChunk(sink, chunk)
	}
}

func (v *View) publishChunk(sink BatchSink, chunk ChunkCoord) {
	// BatchPort clones the instances, so the buffer is safe to reuse.
	v.buffer = v.Layer.Chunk(chunk.X, chunk.Y, v.Atlas, v.buffer)
	sink.UpdateSprites(v.batchID(chunk), v.buffer)
}

// Commands appends one DrawSpriteBatch per visible chunk to dst. It culls
// with the layer's parallax camera, the same camera the batches render with.
func (v *View) Commands(dst []render.Command, cam render.Camera, layer int) []render.Command {
	v.chunks = visibleChunksInto(v.chunks[:0], &v.Layer, cam.ForLayer(layer), v.Atlas)
	for _, chunk := range v.chunks {
		dst = append(dst, render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Batch:       v.batchID(chunk),
		})
	}
	return dst
}

func (v *View) batchID(chunk ChunkCoord) render.SpriteBatchID {
	chunksWide := (v.Layer.W-1)/ChunkSize + 1
	return v.base + render.SpriteBatchID(chunk.Y*chunksWide+chunk.X)
}

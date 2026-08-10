package tilemap

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func BenchmarkChunkWithReusableBuffer(b *testing.B) {
	layer := Layer{
		Tiles:    make([]int16, ChunkSize*ChunkSize),
		W:        ChunkSize,
		H:        ChunkSize,
		CellSize: math2d.Vec2{X: 1, Y: 1},
	}
	atlas := Atlas{Texture: 1, TileSize: math2d.Vec2{X: 16, Y: 16}, Columns: 8}
	buf := make([]render.SpriteInstance, 0, ChunkSize*ChunkSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = layer.Chunk(0, 0, atlas, buf)
	}
}

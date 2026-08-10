package tilemap

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestChunkMapsCellsToWorldAndAtlas(t *testing.T) {
	layer := Layer{
		Tiles: []int16{
			0, 1, -1, 3,
			4, 5, 6, 7,
		},
		W:        4,
		H:        2,
		Origin:   math2d.Vec2{X: 10, Y: 20},
		CellSize: math2d.Vec2{X: 8, Y: 6},
	}
	atlas := Atlas{
		Texture:  7,
		TileSize: math2d.Vec2{X: 16, Y: 12},
		Columns:  4,
	}

	got := layer.Chunk(0, 0, atlas, make([]render.SpriteInstance, 0, 8))
	want := []render.SpriteInstance{
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 16, Y: 12})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 18, Y: 20}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{X: 16, Y: 0}, math2d.Vec2{X: 16, Y: 12})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 34, Y: 20}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{X: 48, Y: 0}, math2d.Vec2{X: 16, Y: 12})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 10, Y: 26}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{X: 0, Y: 12}, math2d.Vec2{X: 16, Y: 12})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 18, Y: 26}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{X: 16, Y: 12}, math2d.Vec2{X: 16, Y: 12})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 26, Y: 26}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{X: 32, Y: 12}, math2d.Vec2{X: 16, Y: 12})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 34, Y: 26}, math2d.Vec2{X: 8, Y: 6}), Src: math2d.NewRect(math2d.Vec2{X: 48, Y: 12}, math2d.Vec2{X: 16, Y: 12})},
	}
	for i := range want {
		want[i].ProjectCorners = true
	}

	if len(got) != len(want) {
		t.Fatalf("chunk length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chunk[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestChunkReusesBuffer(t *testing.T) {
	layer := Layer{
		Tiles:    make([]int16, ChunkSize*ChunkSize),
		W:        ChunkSize,
		H:        ChunkSize,
		CellSize: math2d.Vec2{X: 1, Y: 1},
	}
	atlas := Atlas{TileSize: math2d.Vec2{X: 1, Y: 1}, Columns: 1}
	buf := make([]render.SpriteInstance, 0, ChunkSize*ChunkSize)
	got := layer.Chunk(0, 0, atlas, buf)
	if len(got) != cap(buf) {
		t.Fatalf("chunk length = %d, want %d", len(got), cap(buf))
	}
	if &got[0] != &buf[:cap(buf)][0] {
		t.Fatal("chunk did not reuse the supplied buffer")
	}
}

func TestChunkSkipsInvalidAndOutOfRangeCells(t *testing.T) {
	layer := Layer{Tiles: []int16{-1, 2}, W: 2, H: 1, CellSize: math2d.Vec2{X: 1, Y: 1}}
	atlas := Atlas{TileSize: math2d.Vec2{X: 1, Y: 1}, Columns: 1}
	buf := make([]render.SpriteInstance, 0, 2)

	if got := layer.Chunk(-1, 0, atlas, buf); len(got) != 0 {
		t.Fatalf("negative chunk length = %d, want 0", len(got))
	}
	if got := layer.Chunk(1, 0, atlas, buf); len(got) != 0 {
		t.Fatalf("outside chunk length = %d, want 0", len(got))
	}
	if got := layer.Chunk(0, 0, atlas, buf); len(got) != 1 {
		t.Fatalf("non-empty cell count = %d, want 1", len(got))
	}
}

func TestSetTileTracksChangedChunks(t *testing.T) {
	layer := Layer{Tiles: make([]int16, 2*ChunkSize*ChunkSize), W: 2 * ChunkSize, H: ChunkSize}

	layer.SetTile(0, 0, 1)
	layer.SetTile(ChunkSize, 0, 2)
	layer.SetTile(0, 0, 1)
	got := layer.DirtyChunks()
	want := []ChunkCoord{{X: 0, Y: 0}, {X: 1, Y: 0}}
	if len(got) != len(want) {
		t.Fatalf("dirty chunk count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dirty[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
	if got := layer.DirtyChunks(); got != nil {
		t.Fatalf("dirty chunks after consume = %#v, want nil", got)
	}

	layer.SetTile(-1, 0, 3)
	layer.SetTile(layer.W, 0, 3)
	if got := layer.DirtyChunks(); got != nil {
		t.Fatalf("out-of-range dirty chunks = %#v, want nil", got)
	}
}

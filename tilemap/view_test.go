package tilemap

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type recordingSink struct {
	updates map[render.SpriteBatchID]int
	sprites map[render.SpriteBatchID][]render.SpriteInstance
}

func (s *recordingSink) UpdateSprites(id render.SpriteBatchID, instances []render.SpriteInstance) {
	if s.updates == nil {
		s.updates = make(map[render.SpriteBatchID]int)
	}
	s.updates[id]++
	if s.sprites == nil {
		s.sprites = make(map[render.SpriteBatchID][]render.SpriteInstance)
	}
	s.sprites[id] = append(s.sprites[id][:0], instances...)
}

func TestViewReportsOccupiedBatchIDs(t *testing.T) {
	if got := viewFixture().Batches(); got != 4 {
		t.Fatalf("Batches() = %d, want 4", got)
	}
	if got := NewView(Layer{}, Atlas{}, 1).Batches(); got != 0 {
		t.Fatalf("empty Batches() = %d, want 0", got)
	}
}

func viewFixture() *View {
	layer := Layer{
		Tiles:    make([]int16, 2*ChunkSize*2*ChunkSize),
		W:        2 * ChunkSize,
		H:        2 * ChunkSize,
		CellSize: math2d.Vec2{X: 1, Y: 1},
	}
	atlas := Atlas{TileSize: math2d.Vec2{X: 16, Y: 16}, Columns: 4}
	return NewView(layer, atlas, 100)
}

// The first Publish uploads every chunk; later calls upload only the chunks
// changed through the Layer's Set methods, with stable per-chunk batch IDs.
func TestViewPublishUploadsAllThenOnlyDirtyChunks(t *testing.T) {
	view := viewFixture()
	sink := &recordingSink{}

	view.Publish(sink)
	if len(sink.updates) != 4 {
		t.Fatalf("first publish uploaded %d chunks, want 4", len(sink.updates))
	}
	for _, id := range []render.SpriteBatchID{100, 101, 102, 103} {
		if sink.updates[id] != 1 {
			t.Fatalf("batch %d uploads = %d, want 1", id, sink.updates[id])
		}
	}

	sink.updates = nil
	view.Publish(sink)
	if len(sink.updates) != 0 {
		t.Fatalf("clean publish uploaded %d chunks, want 0", len(sink.updates))
	}

	view.Layer.SetTile(ChunkSize, ChunkSize, 3)
	view.Publish(sink)
	if len(sink.updates) != 1 || sink.updates[103] != 1 {
		t.Fatalf("dirty publish uploads = %v, want only batch 103", sink.updates)
	}
}

// Commands must emit the same chunks the low-level VisibleChunks path
// selects, tagged with the caller's layer and the view's batch IDs.
func TestViewCommandsMatchVisibleChunks(t *testing.T) {
	view := viewFixture()
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: 32, Y: 32}, math2d.Vec2{X: 32, Y: 32}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 32, Y: 32}),
	}
	const layer = 7

	const clock = 2.5

	got := view.Commands(nil, cam, layer, clock)
	var want []render.Command
	for _, chunk := range VisibleChunks(&view.Layer, cam.ForLayer(layer), view.Atlas) {
		want = append(want, render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Batch:       view.batchID(chunk),
			// The render clock rides every chunk so atlas flip-books animate.
			Animation: render.SpriteBatchAnimation{Time: clock},
		})
	}
	if len(want) == 0 {
		t.Fatal("fixture camera culled every chunk; the parity check needs at least one")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

// Tile objects stay in authored order across texture changes and publish only
// once. The backends split consecutive texture ranges inside the batch.
func TestObjectViewPreservesOrderAndPublishesOnce(t *testing.T) {
	objects := []TileObject{
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1})},
		{Texture: 9, Dest: math2d.NewRect(math2d.Vec2{X: 2}, math2d.Vec2{X: 1, Y: 1})},
		{Texture: 7, Dest: math2d.NewRect(math2d.Vec2{X: 4}, math2d.Vec2{X: 1, Y: 1}), Animation: render.SpriteAnimation{Frames: 2, Rate: 4}},
	}
	view := NewObjectView(objects, 50)
	sink := &recordingSink{}

	view.Publish(sink)
	if len(sink.updates) != 1 || sink.updates[50] != 1 {
		t.Fatalf("publish uploads = %v, want one upload for batch 50", sink.updates)
	}
	gotTextures := []render.TextureID{sink.sprites[50][0].Texture, sink.sprites[50][1].Texture, sink.sprites[50][2].Texture}
	if wantTextures := []render.TextureID{7, 9, 7}; !reflect.DeepEqual(gotTextures, wantTextures) {
		t.Fatalf("textures = %v, want authored order %v", gotTextures, wantTextures)
	}
	if got := sink.sprites[50][2].Animation; got != (render.SpriteAnimation{Frames: 2, Rate: 4}) {
		t.Fatalf("animation = %+v, want imported flip-book", got)
	}

	sink.updates = nil
	view.Publish(sink)
	if len(sink.updates) != 0 {
		t.Fatalf("second publish uploaded %d batches, want 0", len(sink.updates))
	}

	got := view.Commands(nil, 3, 2.5)
	want := []render.Command{
		render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: 3}, Batch: 50, Animation: render.SpriteBatchAnimation{Time: 2.5}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

// The easy path must keep the outpost optimizations: after the first publish,
// the steady state allocates nothing beyond boxing each emitted command into
// the render.Command interface, a cost every command path pays.
func TestViewSteadyStateDoesNotAllocate(t *testing.T) {
	view := viewFixture()
	sink := &recordingSink{}
	view.Publish(sink)
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
	}
	dst := view.Commands(nil, cam, 0, 0)
	boxed := float64(len(dst))
	if boxed == 0 {
		t.Fatal("fixture camera culled every chunk; the alloc guard needs at least one")
	}

	allocs := testing.AllocsPerRun(100, func() {
		view.Publish(sink)
		dst = view.Commands(dst[:0], cam, 0, 0)
	})
	if allocs > boxed {
		t.Fatalf("steady state allocs = %v, want at most %v (command boxing only)", allocs, boxed)
	}
}

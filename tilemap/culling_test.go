package tilemap

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestVisibleChunksUsesWorldEdgesForIdentityCamera(t *testing.T) {
	layer := Layer{
		Tiles:    make([]int16, 96*96),
		W:        96,
		H:        96,
		CellSize: math2d.Vec2{X: 1, Y: 1},
	}
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: 32, Y: 32}, math2d.Vec2{X: 32, Y: 32}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 32, Y: 32}),
	}

	got := VisibleChunks(&layer, cam)
	want := []ChunkCoord{{X: 1, Y: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visible chunks = %v, want %v", got, want)
	}
}

func TestVisibleChunksAppliesIsometricViewBeforeCulling(t *testing.T) {
	layer := Layer{
		Tiles:    make([]int16, 128*128),
		W:        128,
		H:        128,
		CellSize: math2d.Vec2{X: 1, Y: 1},
	}
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		View: math2d.Affine{
			A: 1, B: 0.5, C: -1, D: 0.5, E: -50,
		},
	}

	got := VisibleChunks(&layer, cam)
	want := []ChunkCoord{
		{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0},
		{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visible chunks = %v, want %v", got, want)
	}
}

func TestVisibleChunksReturnsNilForInvalidLayerOrCamera(t *testing.T) {
	layer := Layer{Tiles: make([]int16, 4), W: 2, H: 2, CellSize: math2d.Vec2{X: 1, Y: 1}}
	invalidCamera := render.Camera{World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1, Y: 0})}
	if got := VisibleChunks(nil, invalidCamera); got != nil {
		t.Fatalf("nil layer chunks = %v, want nil", got)
	}
	if got := VisibleChunks(&layer, invalidCamera); got != nil {
		t.Fatalf("invalid camera chunks = %v, want nil", got)
	}
	layer.Tiles = layer.Tiles[:1]
	if got := VisibleChunks(&layer, render.Camera{World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2})}); got != nil {
		t.Fatalf("malformed layer chunks = %v, want nil", got)
	}
}

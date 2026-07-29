package enginesplash

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestEngineStarfieldScaleMatchesPosterPercentageLayout(t *testing.T) {
	tests := []struct {
		name string
		size math2d.Vec2
		want math2d.Vec2
	}{
		{
			name: "canonical widescreen",
			size: math2d.Vec2{X: 1920, Y: 1080},
			want: math2d.Vec2{X: 1, Y: 1},
		},
		{
			name: "square",
			size: math2d.Vec2{X: 1080, Y: 1080},
			want: math2d.Vec2{X: 0.5625, Y: 1},
		},
		{
			name: "ultrawide",
			size: math2d.Vec2{X: 4320, Y: 1080},
			want: math2d.Vec2{X: 2.25, Y: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewport := math2d.NewRect(math2d.Vec2{}, tt.size)
			got := EngineStarfieldScale(viewport)
			if math.Abs(got.X-tt.want.X) > 1e-12 || math.Abs(got.Y-tt.want.Y) > 1e-12 {
				t.Fatalf("EngineStarfieldScale(%v) = %v, want %v", tt.size, got, tt.want)
			}
			if got.X*engineSkyExtentX != 0.75*viewport.Width() ||
				got.Y*engineSkyExtentY != 0.75*viewport.Height() {
				t.Fatalf("scaled starfield %v does not match poster percentages for %v", got, tt.size)
			}
		})
	}
}

func TestEngineStarfieldSeedMatchesStatic1080pPoster(t *testing.T) {
	want := (uint64(1920)<<32 | uint64(1080)) ^ uint64(2)
	for _, size := range []math2d.Vec2{
		{X: 1920, Y: 1080},
		{X: 1080, Y: 1080},
		{X: 4320, Y: 1080},
	} {
		viewport := math2d.NewRect(math2d.Vec2{}, size)
		if got := EngineStarfieldSeed(viewport); got != want {
			t.Fatalf("EngineStarfieldSeed(%v) = %d, want %d", size, got, want)
		}
	}
}

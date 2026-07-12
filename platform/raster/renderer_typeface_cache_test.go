package raster

import (
	"testing"

	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"golang.org/x/image/font/gofont/goregular"
)

func TestRendererCachesTypefaceAcrossFrames(t *testing.T) {
	parsed, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse goregular: %v", err)
	}
	renderer := Renderer{
		Width:  80,
		Height: 40,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 80, Y: 40}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 80, Y: 40}),
		},
		Fonts: typeface.Registry{nil, parsed},
	}
	command := render.DrawText{
		Position: math2d.Vec2{X: 4, Y: 4},
		Text:     "cached",
		Color:    render.RGBA(1, 1, 1, 1),
		Size:     18,
		Font:     1,
		Baseline: render.BaselineTop,
	}

	renderer.Render([]render.Command{command})
	if len(renderer.faceCache) != 1 {
		t.Fatalf("cache size after first frame = %d, want 1", len(renderer.faceCache))
	}
	firstFace := renderer.faceCache[faceKey{id: 1, px: 18}]
	if firstFace == nil {
		t.Fatal("first frame did not cache the expected face")
	}

	renderer.Render([]render.Command{command})
	if len(renderer.faceCache) != 1 {
		t.Fatalf("cache size after second frame = %d, want 1", len(renderer.faceCache))
	}
	if renderer.faceCache[faceKey{id: 1, px: 18}] != firstFace {
		t.Fatal("second frame replaced the cached face")
	}
}

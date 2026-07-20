package raster_test

import (
	"bytes"
	"image"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestRendererRenderIntoMatchesRenderAndClearsTarget(t *testing.T) {
	renderer := newTestRenderer()
	commands := []render.Command{
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Center:      math2d.Vec2{X: 5, Y: 5},
			Radius:      3,
			Fill:        render.RGBA(1, 0, 0, 1),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 0},
			Rect:        math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 8, Y: 3}),
			Fill:        render.RGBA(0, 0, 1, 1),
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			From:        math2d.Vec2{X: 0, Y: 9},
			To:          math2d.Vec2{X: 9, Y: 0},
			Width:       1,
			Color:       render.RGBA(0, 1, 0, 1),
		},
	}

	want := renderer.Render(commands)
	target := renderer.NewTarget()
	renderer.RenderInto(target, commands)
	if !bytes.Equal(target.Pix, want.Pix) {
		t.Fatal("RenderInto output differs from Render")
	}

	renderer.RenderInto(target, nil)
	cleared := renderer.Render(nil)
	if !bytes.Equal(target.Pix, cleared.Pix) {
		t.Fatal("second RenderInto call retained pixels from the prior frame")
	}
}

func TestRendererRenderIntoRejectsWrongTargetBounds(t *testing.T) {
	renderer := newTestRenderer()
	wrongTarget := image.NewRGBA(image.Rect(0, 0, renderer.Width-1, renderer.Height))

	defer func() {
		if recover() == nil {
			t.Fatal("RenderInto did not panic for a target with wrong dimensions")
		}
	}()

	renderer.RenderInto(wrongTarget, nil)
}

package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestDrawTrianglesRendersAndSkipsInvalidGeometry(t *testing.T) {
	renderer := newTestRenderer()
	vertices := []math2d.Vec2{{X: 1, Y: 1}, {X: 9, Y: 1}, {X: 1, Y: 9}}
	img := renderer.Render([]render.Command{render.DrawTriangles{
		Vertices: vertices, Indices: []uint32{0, 1, 2},
		Color: render.RGBA(1, 0, 0, 1),
	}})
	assertPixel(t, img.At(2, 2), color.RGBA{R: 255, A: 255})

	img = renderer.Render([]render.Command{render.DrawTriangles{
		Vertices: vertices, Indices: []uint32{0, 1, 9},
		Color: render.RGBA(1, 0, 0, 1),
	}})
	assertPixel(t, img.At(2, 2), color.RGBA{A: 255})
}

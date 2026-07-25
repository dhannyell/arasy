package raster

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// goldenImageTexture builds a deterministic 8×8 texture: a 2×2 checkerboard
// of 4 opaque colours, with the bottom-right quadrant semi-transparent (alpha
// ≈ 50 %, premultiplied).
//
//	red          teal
//	(0,0)-(4,4)  (4,0)-(8,4)
//
//	purple       gold (50 %)
//	(0,4)-(4,8)  (4,4)-(8,8)
func goldenImageTexture() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))

	// Top-left: red opaque.
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 220, G: 50, B: 32, A: 255})
		}
	}
	// Top-right: teal opaque.
	for y := 0; y < 4; y++ {
		for x := 4; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 30, G: 180, B: 190, A: 255})
		}
	}
	// Bottom-left: purple opaque.
	for y := 4; y < 8; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 140, G: 60, B: 200, A: 255})
		}
	}
	// Bottom-right: gold semi-transparent (premultiplied: straight {200,170,40} × A 128/255).
	for y := 4; y < 8; y++ {
		for x := 4; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 100, G: 85, B: 20, A: 128})
		}
	}
	return img
}

func TestGoldenImages(t *testing.T) {
	r := goldenRenderer()
	r.Textures = pixel.Registry{nil, goldenImageTexture()}

	img := r.Render([]render.Command{
		// 1 — Scaled up 2.5× (8×8 texture → 20×20 world rect).
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 5, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
		},
		// 2 — Src sub-rect: bottom-left quadrant of the texture (purple).
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 30, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
			Src:     math2d.Rect{Min: math2d.Vec2{X: 0, Y: 4}, Max: math2d.Vec2{X: 4, Y: 8}},
		},
		// 3 — Tinted: green-blue tint at 70 % opacity over the full texture.
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 55, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
			Tint:    render.RGBA(0.2, 0.8, 0.4, 0.7),
		},
		// 4 — Rotated ~30° around the center of Dest.
		render.DrawImage{
			Texture:  1,
			Dest:     math2d.NewRect(math2d.Vec2{X: 5, Y: 35}, math2d.Vec2{X: 25, Y: 25}),
			Rotation: math.Pi / 6,
		},
		// 5 — Missing texture (ID 99) → magenta placeholder.
		render.DrawImage{
			Texture: 99,
			Dest:    math2d.NewRect(math2d.Vec2{X: 38, Y: 35}, math2d.Vec2{X: 20, Y: 20}),
		},
	})

	assertGolden(t, "images", img)
}

func TestGoldenImageQuads(t *testing.T) {
	r := goldenRenderer()
	r.Textures = pixel.Registry{nil, goldenImageTexture()}

	img := r.Render([]render.Command{
		// 1 — Plain rectangle quad (corners of an axis-aligned rect):
		// reproduces a DrawImage blit through the homography path.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 5, Y: 5}, {X: 25, Y: 5}, {X: 25, Y: 25}, {X: 5, Y: 25},
			},
		},
		// 2 — Trapezoid: narrow top edge, wide bottom edge.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 38, Y: 8}, {X: 52, Y: 8}, {X: 60, Y: 28}, {X: 30, Y: 28},
			},
		},
		// 3 — Quad with genuine perspective (non-parallelogram): converging
		// left edge, to exercise the projective (non-affine) homography path.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 68, Y: 5}, {X: 92, Y: 10}, {X: 92, Y: 26}, {X: 68, Y: 28},
			},
		},
		// 4 — Src sub-rect + Tint, on a plain rectangle quad.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 5, Y: 35}, {X: 25, Y: 35}, {X: 25, Y: 55}, {X: 5, Y: 55},
			},
			Src:  math2d.Rect{Min: math2d.Vec2{X: 4, Y: 0}, Max: math2d.Vec2{X: 8, Y: 4}},
			Tint: render.RGBA(0.3, 0.9, 0.6, 0.75),
		},
		// 5 — Missing texture (ID 99) on a rotated (non-rectangular)
		// quad -> magenta placeholder polygon.
		render.DrawImageQuad{
			Texture: 99,
			Corners: [4]math2d.Vec2{
				{X: 45, Y: 40}, {X: 65, Y: 34}, {X: 72, Y: 54}, {X: 50, Y: 60},
			},
		},
		// 6 — Degenerate corners (three collinear points): skipped entirely,
		// nothing drawn here.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 80, Y: 40}, {X: 86, Y: 40}, {X: 92, Y: 40}, {X: 86, Y: 60},
			},
		},
	})

	assertGolden(t, "image_quads", img)
}

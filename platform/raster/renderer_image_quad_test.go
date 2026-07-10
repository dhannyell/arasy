package raster_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// TestDrawImageQuadMatchesDrawImage is the equivalence anchor for the whole
// feature: a DrawImageQuad whose four corners are exactly the corners of a
// rectangle must produce a pixel-identical buffer to the equivalent
// DrawImage (same Dest, no rotation). If this test ever fails, the inverse
// homography sampling has drifted from the affine blit it is meant to
// generalize.
func TestDrawImageQuadMatchesDrawImage(t *testing.T) {
	tex := makeTex2x2()
	dest := math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2})

	rectRenderer := imageRenderer(4, 4)
	rectRenderer.Textures = pixel.Registry{nil, tex}
	rectImg := rectRenderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    dest,
		},
	})

	quadRenderer := imageRenderer(4, 4)
	quadRenderer.Textures = pixel.Registry{nil, tex}
	quadImg := quadRenderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 1,
			Corners: rectCorners(dest),
		},
	})

	if !bytes.Equal(rectImg.Pix, quadImg.Pix) {
		t.Fatalf("DrawImageQuad on a rectangle diverged from DrawImage:\nDrawImage: %v\nDrawImageQuad: %v", rectImg.Pix, quadImg.Pix)
	}
}

// TestDrawImageQuadMatchesDrawImageScaled repeats the equivalence check with
// a texture scaled up into a larger destination, exercising the
// nearest-neighbor scale path (not just a 1:1 blit).
func TestDrawImageQuadMatchesDrawImageScaled(t *testing.T) {
	tex := makeTex2x2()
	dest := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})

	rectRenderer := imageRenderer(4, 4)
	rectRenderer.Textures = pixel.Registry{nil, tex}
	rectImg := rectRenderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    dest,
		},
	})

	quadRenderer := imageRenderer(4, 4)
	quadRenderer.Textures = pixel.Registry{nil, tex}
	quadImg := quadRenderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 1,
			Corners: rectCorners(dest),
		},
	})

	if !bytes.Equal(rectImg.Pix, quadImg.Pix) {
		t.Fatalf("DrawImageQuad on a scaled rectangle diverged from DrawImage:\nDrawImage: %v\nDrawImageQuad: %v", rectImg.Pix, quadImg.Pix)
	}
}

// rectCorners returns the four corners of rect in the top-left, top-right,
// bottom-right, bottom-left order DrawImageQuad expects.
func rectCorners(rect math2d.Rect) [4]math2d.Vec2 {
	return [4]math2d.Vec2{
		{X: rect.Min.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Max.Y},
		{X: rect.Min.X, Y: rect.Max.Y},
	}
}

// TestDrawImageQuadTrapezoid maps a solid texture onto a trapezoid (narrow
// top, wide bottom) and checks that the left/right slanted edges stay
// straight: a handful of key points on each side of the boundary, at three
// different rows, must land inside or outside the quad as predicted by
// linear interpolation between the corners.
func TestDrawImageQuadTrapezoid(t *testing.T) {
	tex := makeTexWhite()
	renderer := imageRenderer(8, 8)
	renderer.Textures = pixel.Registry{nil, tex}

	// Top edge: x in [3,5]. Bottom edge: x in [0,8]. Both edges are
	// straight lines between the corners.
	img := renderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 3, Y: 0}, // top-left
				{X: 5, Y: 0}, // top-right
				{X: 8, Y: 8}, // bottom-right
				{X: 0, Y: 8}, // bottom-left
			},
		},
	})

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	// Row 0 (top, narrow: x in [3,5]).
	assertPixel(t, img.At(4, 0), white) // inside
	assertPixel(t, img.At(0, 0), black) // outside, left of the narrow top
	assertPixel(t, img.At(7, 0), black) // outside, right of the narrow top

	// Row 7 (bottom, wide: x spans nearly the full width).
	assertPixel(t, img.At(4, 7), white) // inside
	assertPixel(t, img.At(0, 7), white) // inside, left edge widened by the trapezoid

	// Row 3 (middle band): left edge ~1.7, right edge ~6.3 (linear
	// interpolation between the corners) — a point comfortably inside and
	// two points comfortably outside on either side.
	assertPixel(t, img.At(4, 3), white) // inside
	assertPixel(t, img.At(0, 3), black) // outside, left of the slanted edge
	assertPixel(t, img.At(7, 3), black) // outside, right of the slanted edge
}

// TestDrawImageQuadSrcAndTint exercises a Src sub-rect together with a Tint,
// mirroring TestDrawImageTint's expected premultiplied math but through the
// quad/homography path. The sampled sub-rect (bottom-right quadrant) is a
// single solid color, so the exact sub-pixel sample position cannot change
// the outcome — this isolates the Src+Tint composition from the sampling
// math already covered by the equivalence tests.
func TestDrawImageQuadSrcAndTint(t *testing.T) {
	tex := makeTex4x4Quadrants()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 1,
			Corners: rectCorners(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})),
			Src:     math2d.Rect{Min: math2d.Vec2{X: 2, Y: 2}, Max: math2d.Vec2{X: 4, Y: 4}}, // yellow quadrant
			Tint:    render.RGBA(1, 0, 0, 0.5),
		},
	})

	// White{255,255,0,255} tinted by {r:0.5,g:0,b:0,a:0.5} premultiplied to
	// {128,0,0,128}, then composited over opaque black clear -> {128,0,0,255}.
	want := color.RGBA{R: 128, G: 0, B: 0, A: 255}
	assertPixel(t, img.At(0, 0), want)
	assertPixel(t, img.At(3, 3), want)
}

// TestDrawImageQuadDegenerateCornersSkipsDraw asserts that a collinear
// (degenerate) corner set draws nothing: the whole buffer stays at the
// clear color.
func TestDrawImageQuadDegenerateCornersSkipsDraw(t *testing.T) {
	tex := makeTexWhite()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 0, Y: 0},
				{X: 2, Y: 0},
				{X: 4, Y: 0},
				{X: 1, Y: 0},
			},
		},
	})

	clear := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	cleared := image.NewRGBA(img.Bounds())
	for y := cleared.Bounds().Min.Y; y < cleared.Bounds().Max.Y; y++ {
		for x := cleared.Bounds().Min.X; x < cleared.Bounds().Max.X; x++ {
			cleared.SetRGBA(x, y, clear)
		}
	}

	if !bytes.Equal(cleared.Pix, img.Pix) {
		t.Fatalf("degenerate quad drew something: %v", img.Pix)
	}
}

// TestDrawImageQuadNonConvexSkipsDraw asserts that a self-intersecting
// (bowtie) corner set draws nothing, even though the corners are not
// collinear and could otherwise produce an invertible homography.
func TestDrawImageQuadNonConvexSkipsDraw(t *testing.T) {
	tex := makeTexWhite()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 1,
			// Listing the same four rectangle corners out of cyclic order
			// (top-left, bottom-right, bottom-left, top-right) connects them
			// with two crossing diagonals instead of the four rectangle
			// edges, producing a self-intersecting "bowtie" quad.
			Corners: [4]math2d.Vec2{
				{X: 0, Y: 0},
				{X: 4, Y: 4},
				{X: 0, Y: 4},
				{X: 4, Y: 0},
			},
		},
	})

	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	assertPixel(t, img.At(0, 0), black)
	assertPixel(t, img.At(1, 1), black)
	assertPixel(t, img.At(2, 2), black)
	assertPixel(t, img.At(3, 3), black)
}

// TestDrawImageQuadMissingTexture renders the magenta placeholder polygon
// when the texture id is unknown.
func TestDrawImageQuadMissingTexture(t *testing.T) {
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil} // no textures loaded

	img := renderer.Render([]render.Command{
		render.DrawImageQuad{
			Texture: 42, // out of range
			Corners: rectCorners(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})),
		},
	})

	magenta := color.RGBA{R: 255, G: 0, B: 255, A: 255}
	assertPixel(t, img.At(0, 0), magenta)
	assertPixel(t, img.At(3, 3), magenta)
}

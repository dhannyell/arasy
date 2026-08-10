package raster_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

// makeTex2x2 builds a 2×2 texture with one distinct color per pixel.
//
//	red   green
//	blue  yellow
func makeTex2x2() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img.SetRGBA(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	img.SetRGBA(0, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 255, G: 255, B: 0, A: 255})
	return img
}

// makeTexFlipSheet builds two 2×2 frames for flip-book tests.
func makeTexFlipSheet() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	frames := [2][2][2]color.RGBA{
		{
			{{R: 255, A: 255}, {G: 255, A: 255}},
			{{B: 255, A: 255}, {R: 255, G: 255, A: 255}},
		},
		{
			{{R: 255, B: 255, A: 255}, {G: 255, B: 255, A: 255}},
			{{R: 255, G: 255, B: 255, A: 255}, {A: 255}},
		},
	}
	for frame, pixels := range frames {
		for y, row := range pixels {
			for x, pixel := range row {
				img.SetRGBA(frame*2+x, y, pixel)
			}
		}
	}
	return img
}

// makeTex4x4Quadrants builds a 4×4 texture with four distinct quadrants.
//
//	red   red   green  green
//	red   red   green  green
//	blue  blue  yellow yellow
//	blue  blue  yellow yellow
func makeTex4x4Quadrants() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			var c color.RGBA
			switch {
			case x < 2 && y < 2:
				c = color.RGBA{R: 255, G: 0, B: 0, A: 255} // red
			case x >= 2 && y < 2:
				c = color.RGBA{R: 0, G: 255, B: 0, A: 255} // green
			case x < 2 && y >= 2:
				c = color.RGBA{R: 0, G: 0, B: 255, A: 255} // blue
			default:
				c = color.RGBA{R: 255, G: 255, B: 0, A: 255} // yellow
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// makeTexWhite builds a 2×2 texture of fully opaque white pixels.
func makeTexWhite() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	w := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	img.SetRGBA(0, 0, w)
	img.SetRGBA(1, 0, w)
	img.SetRGBA(0, 1, w)
	img.SetRGBA(1, 1, w)
	return img
}

func imageRenderer(w, h int) raster.Renderer {
	return raster.Renderer{
		Width:  w,
		Height: h,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(w), Y: float64(h)}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(w), Y: float64(h)}),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}
}

// TestDrawImageBlit1to1 copies every texel 1:1 at an integer position.
func TestDrawImageBlit1to1(t *testing.T) {
	tex := makeTex2x2()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2}),
		},
	})

	// Destination pixels (1,1)-(2,2) carry the four texture colors.
	assertPixel(t, img.At(1, 1), color.RGBA{R: 255, G: 0, B: 0, A: 255})   // red
	assertPixel(t, img.At(2, 1), color.RGBA{R: 0, G: 255, B: 0, A: 255})   // green
	assertPixel(t, img.At(1, 2), color.RGBA{R: 0, G: 0, B: 255, A: 255})   // blue
	assertPixel(t, img.At(2, 2), color.RGBA{R: 255, G: 255, B: 0, A: 255}) // yellow

	// Pixels outside the dest stay clear.
	assertPixel(t, img.At(0, 0), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(3, 3), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestSpriteFlipMatchesRotationAndFlipBookOrder(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	green := color.RGBA{G: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	yellow := color.RGBA{R: 255, G: 255, A: 255}
	magenta := color.RGBA{R: 255, B: 255, A: 255}
	cyan := color.RGBA{G: 255, B: 255, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}

	cases := []struct {
		name         string
		flip         render.SpriteFlip
		static       [2][2]color.RGBA
		staticTurn   [2][2]color.RGBA
		animated     [2][2]color.RGBA
		animatedTurn [2][2]color.RGBA
	}{
		{
			name:         "none",
			flip:         render.FlipNone,
			static:       [2][2]color.RGBA{{red, green}, {blue, yellow}},
			staticTurn:   [2][2]color.RGBA{{blue, red}, {yellow, green}},
			animated:     [2][2]color.RGBA{{magenta, cyan}, {white, black}},
			animatedTurn: [2][2]color.RGBA{{white, magenta}, {black, cyan}},
		},
		{
			name:         "horizontal",
			flip:         render.FlipX,
			static:       [2][2]color.RGBA{{green, red}, {yellow, blue}},
			staticTurn:   [2][2]color.RGBA{{yellow, green}, {blue, red}},
			animated:     [2][2]color.RGBA{{cyan, magenta}, {black, white}},
			animatedTurn: [2][2]color.RGBA{{black, cyan}, {white, magenta}},
		},
		{
			name:         "vertical",
			flip:         render.FlipY,
			static:       [2][2]color.RGBA{{blue, yellow}, {red, green}},
			staticTurn:   [2][2]color.RGBA{{red, blue}, {green, yellow}},
			animated:     [2][2]color.RGBA{{white, black}, {magenta, cyan}},
			animatedTurn: [2][2]color.RGBA{{magenta, white}, {cyan, black}},
		},
		{
			name:         "both",
			flip:         render.FlipX | render.FlipY,
			static:       [2][2]color.RGBA{{yellow, blue}, {green, red}},
			staticTurn:   [2][2]color.RGBA{{green, yellow}, {red, blue}},
			animated:     [2][2]color.RGBA{{black, white}, {cyan, magenta}},
			animatedTurn: [2][2]color.RGBA{{cyan, black}, {magenta, white}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFlipPixels(t, renderFlipImage(tc.flip, 0, false), tc.static)
			assertFlipPixels(t, renderFlipImage(tc.flip, math.Pi/2, false), tc.staticTurn)
			assertFlipPixels(t, renderFlipImage(tc.flip, 0, true), tc.animated)
			assertFlipPixels(t, renderFlipImage(tc.flip, math.Pi/2, true), tc.animatedTurn)
		})
	}
}

func renderFlipImage(flip render.SpriteFlip, rotation float64, animated bool) *image.RGBA {
	renderer := imageRenderer(2, 2)
	renderer.Textures = pixel.Registry{nil, makeTexFlipSheet()}
	dest := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2})
	if !animated {
		return renderer.Render([]render.Command{render.DrawImage{
			Texture:  1,
			Dest:     dest,
			Src:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}),
			Rotation: rotation,
			Flip:     flip,
		}})
	}
	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{{
		Texture:   1,
		Dest:      dest,
		Src:       math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}),
		Rotation:  rotation,
		Flip:      flip,
		Animation: render.SpriteAnimation{Frames: 2, Rate: 1},
	}})
	return renderer.Render([]render.Command{render.DrawSpriteBatch{
		Batch:     1,
		Animation: render.SpriteBatchAnimation{Time: 1},
	}})
}

func assertFlipPixels(t *testing.T, img *image.RGBA, want [2][2]color.RGBA) {
	t.Helper()
	for y, row := range want {
		for x, pixel := range row {
			if got := img.RGBAAt(x, y); got != pixel {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, pixel)
			}
		}
	}
}

// TestDrawImageTransformRotatesTexturedQuad proves a DrawImage carrying a
// non-identity Transform is inverse-sampled onto the transformed quad (like the
// Canvas2D/WebGPU backends) instead of the old axis-aligned magenta
// placeholder. A 45°-rotated 8×8 white sprite reaches above its axis-aligned
// destination top edge, and its former axis-aligned corner becomes clear.
func TestDrawImageTransformRotatesTexturedQuad(t *testing.T) {
	tex := makeTexWhite()
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, tex}

	center := math2d.Vec2{X: 8, Y: 8}
	img := renderer.Render([]render.Command{
		render.DrawImage{
			BaseCommand: render.BaseCommand{Transform: math2d.Rotate(math.Pi / 4).About(center)},
			Texture:     1,
			Dest:        math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 8, Y: 8}),
		},
	})

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	magenta := color.RGBA{R: 255, G: 0, B: 255, A: 255}

	// (8,3) is above the axis-aligned dest (y<4) but inside the rotated diamond,
	// so only a truly rotated sprite paints it white.
	assertPixel(t, img.At(8, 3), white)
	// The rotated sprite must not be the magenta missing-texture placeholder.
	if got := img.At(8, 8); got == magenta {
		t.Fatal("transformed image drew the magenta placeholder over a valid texture")
	}
	// (4,4) is a corner of the axis-aligned dest — an unrotated blit would paint
	// it white, but the 45° rotation retracts the corner, leaving it clear.
	assertPixel(t, img.At(4, 4), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

// TestDrawImageScaleNearest scales a 2×2 texture to fill 4×4 — each texel
// becomes a 2×2 block.
func TestDrawImageScaleNearest(t *testing.T) {
	tex := makeTex2x2()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		},
	})

	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}

	for _, tc := range []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, red}, {1, 0, red}, {0, 1, red}, {1, 1, red},
		{2, 0, green}, {3, 0, green}, {2, 1, green}, {3, 1, green},
		{0, 2, blue}, {1, 2, blue}, {0, 3, blue}, {1, 3, blue},
		{2, 2, yellow}, {3, 2, yellow}, {2, 3, yellow}, {3, 3, yellow},
	} {
		assertPixel(t, img.At(tc.x, tc.y), tc.want)
	}
}

// TestDrawImageSrcSubRect draws only the bottom-right quadrant of a 4×4
// texture by specifying Src.
func TestDrawImageSrcSubRect(t *testing.T) {
	tex := makeTex4x4Quadrants()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	// Bottom-right quadrant: yellow.
	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}),
			Src:     math2d.Rect{Min: math2d.Vec2{X: 2, Y: 2}, Max: math2d.Vec2{X: 4, Y: 4}},
		},
	})

	yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	assertPixel(t, img.At(0, 0), yellow)
	assertPixel(t, img.At(1, 0), yellow)
	assertPixel(t, img.At(0, 1), yellow)
	assertPixel(t, img.At(1, 1), yellow)

	// Pixels outside the dest remain clear.
	assertPixel(t, img.At(2, 0), black)
	assertPixel(t, img.At(0, 2), black)
}

// TestDrawImageTintMultipliesChannels applies a red semi-transparent tint to
// a white texture and verifies the premultiplied blended result.
//
// White pixel {255,255,255,255} × tint factors {r:0.5, g:0, b:0, a:0.5}
// produces premultiplied pixel {128,0,0,128}.
// Composited over clear black {0,0,0,255} → {128,0,0,255}.
func TestDrawImageTint(t *testing.T) {
	tex := makeTexWhite()
	renderer := imageRenderer(2, 2)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}),
			Tint:    render.RGBA(1, 0, 0, 0.5),
		},
	})

	want := color.RGBA{R: 128, G: 0, B: 0, A: 255}
	assertPixel(t, img.At(0, 0), want)
	assertPixel(t, img.At(1, 1), want)
}

// TestDrawImageTintZero is identical to 1:1 blit — a zero Color is treated
// as noTint and leaves the texture unchanged.
func TestDrawImageTintZero(t *testing.T) {
	tex := makeTex2x2()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2}),
			Tint:    render.Color{}, // zero value = no tint
		},
	})

	assertPixel(t, img.At(1, 1), color.RGBA{R: 255, G: 0, B: 0, A: 255})   // red
	assertPixel(t, img.At(2, 1), color.RGBA{R: 0, G: 255, B: 0, A: 255})   // green
	assertPixel(t, img.At(1, 2), color.RGBA{R: 0, G: 0, B: 255, A: 255})   // blue
	assertPixel(t, img.At(2, 2), color.RGBA{R: 255, G: 255, B: 0, A: 255}) // yellow
}

// TestDrawImageMissingTexture renders the magenta placeholder for out-of-range
// and zero TextureIDs.
func TestDrawImageMissingTexture(t *testing.T) {
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil} // no textures loaded

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture: 42, // out of range
			Dest:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}),
		},
		render.DrawImage{
			Texture: 0, // special "missing" id
			Dest:    math2d.NewRect(math2d.Vec2{X: 2, Y: 0}, math2d.Vec2{X: 2, Y: 2}),
		},
	})

	magenta := color.RGBA{R: 255, G: 0, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	// Both destinations show magenta.
	assertPixel(t, img.At(0, 0), magenta)
	assertPixel(t, img.At(1, 1), magenta)
	assertPixel(t, img.At(2, 0), magenta)
	assertPixel(t, img.At(3, 1), magenta)

	// Pixel outside both dests stays clear.
	assertPixel(t, img.At(0, 3), black)
}

// TestDrawImageClip restricts drawing to the Clip rect — only the covered
// half of the dest is drawn.
func TestDrawImageClip(t *testing.T) {
	tex := makeTexWhite()
	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImage{
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 4}),
			},
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		},
	})

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	// Left half is drawn.
	assertPixel(t, img.At(0, 0), white)
	assertPixel(t, img.At(1, 2), white)

	// Right half is clipped away.
	assertPixel(t, img.At(2, 0), black)
	assertPixel(t, img.At(3, 3), black)
}

// TestDrawImageRotation180 swaps the two texels of a 2×1 texture when
// rotated by π around the center of Dest, confirming the pivot is correct.
func TestDrawImageRotation180(t *testing.T) {
	// 2×1 texture: A=red, B=blue
	tex := image.NewRGBA(image.Rect(0, 0, 2, 1))
	tex.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255}) // A
	tex.SetRGBA(1, 0, color.RGBA{R: 0, G: 0, B: 255, A: 255}) // B

	renderer := imageRenderer(4, 4)
	renderer.Textures = pixel.Registry{nil, tex}

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture:  1,
			Dest:     math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 1}),
			Rotation: math.Pi,
		},
	})

	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	// After π rotation: screen (0,0) → src(1,0)=B, screen (1,0) → src(0,0)=A.
	assertPixel(t, img.At(0, 0), blue)
	assertPixel(t, img.At(1, 0), red)

	// Pixels outside the dest remain clear.
	assertPixel(t, img.At(2, 0), black)
	assertPixel(t, img.At(0, 1), black)
}

// TestDrawImageMissingTextureRotated renders the magenta placeholder for a
// missing texture with a rotated Dest. The placeholder must cover the
// rotated quad the image would have occupied, not the unrotated Dest rect:
// a corner of the original axis-aligned rect falls outside the rotated
// square and stays clear, while a point outside that rect but inside the
// rotated square's tip turns magenta.
func TestDrawImageMissingTextureRotated(t *testing.T) {
	renderer := imageRenderer(40, 40)
	renderer.Textures = pixel.Registry{nil} // no textures loaded

	img := renderer.Render([]render.Command{
		render.DrawImage{
			Texture:  42, // out of range
			Dest:     math2d.NewRect(math2d.Vec2{X: 16, Y: 16}, math2d.Vec2{X: 8, Y: 8}),
			Rotation: math.Pi / 4,
		},
	})

	magenta := color.RGBA{R: 255, G: 0, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	// Center of Dest is always covered, rotated or not.
	assertPixel(t, img.At(20, 20), magenta)

	// (19,15) sits above the unrotated Dest (whose top edge is at y=16) but
	// inside the 45°-rotated square's upper tip: only a rotation-aware
	// placeholder paints it.
	assertPixel(t, img.At(19, 15), magenta)

	// (16,16) is a corner of the unrotated Dest rect. A naive axis-aligned
	// placeholder (the old, unrotated behavior) would paint it magenta, but
	// the 45°-rotated square doesn't reach that far into the corner, so it
	// must stay clear.
	assertPixel(t, img.At(16, 16), black)
}

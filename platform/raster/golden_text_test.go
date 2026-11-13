package raster

import (
	"bytes"
	"image"
	"math"
	"testing"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/render/slug"
	"golang.org/x/image/font/gofont/goregular"
)

func TestGoldenText(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 8, Y: 20},
			To:    math2d.Vec2{X: 92, Y: 20},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 0.28),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 8, Y: 44},
			To:    math2d.Vec2{X: 92, Y: 44},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 0.28),
		},
		render.DrawText{
			Position: math2d.Vec2{X: 10, Y: 20},
			Text:     "Left",
			Color:    render.RGBA(0.95, 0.95, 0.95, 1),
			Size:     6.5,
			Align:    render.AlignLeft,
			Baseline: render.BaselineTop,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 50, Y: 20},
			Text:     "Mid",
			Color:    render.RGBA(0.35, 0.9, 1, 1),
			Size:     6.5,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 90, Y: 20},
			Text:     "Right",
			Color:    render.RGBA(1, 0.76, 0.3, 1),
			Size:     6.5,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 16, Y: 50},
			Text:     "2x",
			Color:    render.RGBA(1, 0.4, 0.55, 0.86),
			Size:     13,
			Align:    render.AlignLeft,
			Baseline: render.BaselineAlphabetic,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 80, Y: 58},
			Text:     "",
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     13,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 64, Y: 58},
			Text:     "Tilt",
			Color:    render.RGBA(1, 0.85, 0.25, 1),
			Size:     7.5,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
			Rotation: -math.Pi / 5,
		},
	})

	assertFontZeroGolden(t, "text", img)
}

func TestGoldenTypeface(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse goregular: %v", err)
	}
	renderer := goldenRenderer()
	renderer.Fonts = typeface.Registry{nil, face}
	img := renderer.Render([]render.Command{
		render.DrawText{
			Position: math2d.Vec2{X: 7, Y: 8},
			Text:     "Go Regular 6",
			Color:    render.RGBA(0.92, 0.95, 1, 1),
			Size:     6,
			Font:     1,
			Baseline: render.BaselineTop,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 50, Y: 24},
			Text:     "Center 10",
			Color:    render.RGBA(0.3, 0.88, 1, 1),
			Size:     10,
			Font:     1,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 93, Y: 45},
			Text:     "Right 16",
			Color:    render.RGBA(1, 0.72, 0.25, 1),
			Size:     16,
			Font:     1,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 23, Y: 49},
			Text:     "Rotate",
			Color:    render.RGBA(1, 0.42, 0.63, 0.9),
			Size:     9,
			Font:     1,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
			Rotation: -math.Pi / 7,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 7, Y: 68},
			Text:     "builtin 0",
			Color:    render.RGBA(0.62, 1, 0.48, 1),
			Size:     6,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 93, Y: 67},
			Text:     "fallback 99",
			Color:    render.RGBA(0.82, 0.72, 1, 1),
			Size:     6,
			Font:     99,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
	})

	// x/image's anti-aliased glyph rasterizer can round edge coverage by one
	// channel value across platforms. Keep this tolerance local to the external
	// typeface golden; the remaining raster goldens stay pixel-exact.
	assertFontZeroGoldenTolerance(t, "typeface", img, 1)
}

// The atlas is the reference for placement: Slug must draw the same text in the
// same place, for each path that moves it. Integer sizes avoid the atlas px round.
func TestSlugTextMatchesAtlasPlacement(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse goregular: %v", err)
	}
	base := render.DrawText{
		Position: math2d.Vec2{X: 50, Y: 30},
		Text:     "Hamburg",
		Color:    render.RGBA(1, 1, 1, 1),
		Size:     12,
		Font:     1,
	}
	with := func(change func(*render.DrawText)) render.DrawText {
		c := base
		change(&c)
		return c
	}
	cases := map[string]render.DrawText{
		"alphabetic": base,
		"center":     with(func(c *render.DrawText) { c.Align = render.AlignCenter }),
		"right":      with(func(c *render.DrawText) { c.Align = render.AlignRight }),
		"top":        with(func(c *render.DrawText) { c.Baseline = render.BaselineTop }),
		"middle":     with(func(c *render.DrawText) { c.Baseline = render.BaselineMiddle }),
		"bottom":     with(func(c *render.DrawText) { c.Baseline = render.BaselineBottom }),
		"rotation":   with(func(c *render.DrawText) { c.Rotation = math.Pi / 6; c.Align = render.AlignCenter }),
		"spacing":    with(func(c *render.DrawText) { c.LetterSpacing = 1.5; c.Align = render.AlignRight }),
		"transform": with(func(c *render.DrawText) {
			c.Transform = math2d.Affine{A: 1, B: 0.25, C: -0.25, D: 1, E: 6, F: 2}
		}),
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			renderer := goldenRenderer()
			renderer.Width, renderer.Height = 200, 120
			renderer.Camera = render.Camera{
				World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 60}),
				Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 120}),
			}
			renderer.Fonts = typeface.Registry{nil, face}
			renderer.GlyphOutlines = slug.Outlines
			atlas := renderer.Render([]render.Command{command})
			command.Strategy = render.GlyphSlug
			outlined := renderer.Render([]render.Command{command})

			want, wantInk := inkOf(atlas)
			got, gotInk := inkOf(outlined)
			if wantInk == 0 || gotInk == 0 {
				t.Fatalf("ink: atlas %v, slug %v", wantInk, gotInk)
			}
			const edge = 2 // pixels: the atlas rounds its origin and its box
			if abs := want.Min.Sub(got.Min); max(abs.X, -abs.X, abs.Y, -abs.Y) > edge {
				t.Errorf("min: atlas %v, slug %v", want, got)
			}
			if abs := want.Max.Sub(got.Max); max(abs.X, -abs.X, abs.Y, -abs.Y) > edge {
				t.Errorf("max: atlas %v, slug %v", want, got)
			}
			if ratio := gotInk / wantInk; ratio < 0.95 || ratio > 1.05 {
				t.Errorf("ink ratio = %.3f", ratio)
			}
		})
	}
}

// inkOf returns the bounds and the sum of the alpha of the pixels over a clear image.
func inkOf(img *image.RGBA) (image.Rectangle, float64) {
	background := img.RGBAAt(0, 0)
	var bounds image.Rectangle
	var ink float64
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.G > background.G+32 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
			ink += float64(c.G) - float64(background.G)
		}
	}
	return bounds, ink
}

// A game without GlyphOutlines must still see its GlyphSlug text.
func TestGlyphSlugWithoutOutlinesUsesAtlas(t *testing.T) {
	command := render.DrawText{
		Position: math2d.Vec2{X: 8, Y: 40},
		Text:     "Hamburg",
		Color:    render.RGBA(1, 1, 1, 1),
		Size:     12,
	}
	atlas := goldenRenderer().Render([]render.Command{command})
	command.Strategy = render.GlyphSlug
	got := goldenRenderer().Render([]render.Command{command})
	if !bytes.Equal(got.Pix, atlas.Pix) {
		t.Fatal("GlyphSlug without GlyphOutlines differs from the atlas")
	}
	if _, ink := inkOf(atlas); ink == 0 {
		t.Fatal("the atlas drew nothing")
	}
}

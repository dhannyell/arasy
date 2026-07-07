package tess

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	atlasFirstRune = 32
	atlasLastRune  = 126
	atlasCols      = 16
	glyphWidth     = 7
	glyphHeight    = 13
)

// Atlas stores the bitmap font texture and per-rune UV coordinates.
type Atlas struct {
	Image *image.Alpha
	uv    map[rune][4]float32
}

// TextVertex is one interleaved text vertex: clip-space position, straight
// alpha color, and atlas UV coordinates.
type TextVertex struct {
	X, Y       float32
	R, G, B, A float32
	U, V       float32
}

// TextMesh accumulates triangles for the text pipeline.
type TextMesh struct {
	Vertices []TextVertex
}

// NewAtlas rasterizes the built-in monospace bitmap font into a single alpha
// texture and records UVs for printable ASCII runes.
func NewAtlas() *Atlas {
	rows := (atlasLastRune - atlasFirstRune + atlasCols) / atlasCols
	img := image.NewAlpha(image.Rect(0, 0, atlasCols*glyphWidth, rows*glyphHeight))
	uv := make(map[rune][4]float32, atlasLastRune-atlasFirstRune+1)

	face := basicfont.Face7x13
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Alpha{A: 255}),
		Face: face,
	}

	for r := rune(atlasFirstRune); r <= rune(atlasLastRune); r++ {
		i := int(r) - atlasFirstRune
		col := i % atlasCols
		row := i / atlasCols
		x := col * glyphWidth
		y := row * glyphHeight

		drawer.Dot = fixed.Point26_6{
			X: fixed.I(x),
			Y: fixed.I(y) + face.Metrics().Ascent,
		}
		drawer.DrawString(string(r))

		uv[r] = [4]float32{
			float32(x) / float32(img.Bounds().Dx()),
			float32(y) / float32(img.Bounds().Dy()),
			float32(x+glyphWidth) / float32(img.Bounds().Dx()),
			float32(y+glyphHeight) / float32(img.Bounds().Dy()),
		}
	}

	return &Atlas{Image: img, uv: uv}
}

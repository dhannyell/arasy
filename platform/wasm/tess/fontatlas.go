package tess

import (
	"errors"
	"image"
	"image/color"

	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/render"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	atlasFirstRune = 32
	atlasLastRune  = 126
	atlasCols      = 16
	glyphWidth     = 7
	glyphHeight    = 13
)

// AtlasKey identifies one rasterized font atlas for a concrete screen
// pixel size. Font 0 is the built-in engine bitmap font.
type AtlasKey struct {
	Font render.FontID
	PX   int
}

// Glyph stores one atlas entry: UVs plus pixel-space advance and bounds.
type Glyph struct {
	UV      [4]float32
	Advance float64
	Width   float64
	Height  float64
	OffsetX float64
	OffsetY float64
}

// Atlas stores the bitmap font texture and per-rune UV coordinates.
type Atlas struct {
	Key    AtlasKey
	Image  *image.Alpha
	Glyphs map[rune]Glyph
	Ascent float64
	Height float64
	uv     map[rune][4]float32
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

	return &Atlas{Key: AtlasKey{Font: 0, PX: 13}, Image: img, uv: uv}
}

func NewAtlasForFace(fontID render.FontID, px int, face *typeface.Face, runes []rune) (*Atlas, error) {
	rows := (runes[len(runes)-1] - runes[0] + atlasCols) / atlasCols
	img := image.NewAlpha(image.Rect(0, 0, atlasCols*glyphWidth, int(rows*glyphHeight)))
	uv := make(map[rune][4]float32, atlasLastRune-atlasFirstRune+1)

	fontFace, r := opentype.NewFace(face.Font(), &opentype.FaceOptions{
		Size: float64(px), DPI: 72, Hinting: font.HintingNone,
	})
	if r == nil {
		return nil, errors.New("could not create a new AtlasForFace")
	}

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Alpha{A: 255}),
		Face: fontFace,
	}

	for r := rune(0); r <= rune(atlasLastRune); r++ {
		i := int(r) - atlasFirstRune
		col := i % atlasCols
		row := i / atlasCols
		x := col * glyphWidth
		y := row * glyphHeight

		drawer.Dot = fixed.Point26_6{
			X: fixed.I(x),
			Y: fixed.I(y) + fixed.I(int(face.Ascent(float64(px)))),
		}
		drawer.DrawString(string(r))

		uv[r] = [4]float32{
			float32(x) / float32(img.Bounds().Dx()),
			float32(y) / float32(img.Bounds().Dy()),
			float32(x+glyphWidth) / float32(img.Bounds().Dx()),
			float32(y+glyphHeight) / float32(img.Bounds().Dy()),
		}
	}

	return &Atlas{
		Key:   AtlasKey{Font: fontID, PX: px},
		Image: img,
		uv:    uv,
	}, nil
}

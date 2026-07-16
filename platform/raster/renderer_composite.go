package raster

import (
	"image/color"

	"github.com/dhannyell/frameforge/render"
)

type tintFactors struct {
	r, g, b, a float64
}

var noTint = tintFactors{1, 1, 1, 1}

// toRGBA converts a normalized render.Color into the alpha-premultiplied
// color.RGBA that image.RGBA's pixel format requires.
func toRGBA(c render.Color) color.RGBA {
	return color.RGBA{
		R: toByte(c.R * c.A),
		G: toByte(c.G * c.A),
		B: toByte(c.B * c.A),
		A: toByte(c.A),
	}
}

// blendRGBA composites premultiplied src over premultiplied dst
// (the Porter-Duff "over" operator).
func blendRGBA(dst, src color.RGBA) color.RGBA {
	if src.A == 255 {
		return src
	}
	if src.A == 0 {
		return dst
	}

	inv := uint32(255 - src.A)
	return color.RGBA{
		R: src.R + uint8(uint32(dst.R)*inv/255),
		G: src.G + uint8(uint32(dst.G)*inv/255),
		B: src.B + uint8(uint32(dst.B)*inv/255),
		A: src.A + uint8(uint32(dst.A)*inv/255),
	}
}

func tintFrom(c render.Color) tintFactors {
	if c == (render.Color{}) {
		return noTint
	}
	return tintFactors{r: c.R * c.A, g: c.G * c.A, b: c.B * c.A, a: c.A}
}

func (t tintFactors) apply(c color.RGBA) color.RGBA {
	if t == noTint {
		return c
	}
	return color.RGBA{
		R: toByte(float64(c.R) / 255 * t.r),
		G: toByte(float64(c.G) / 255 * t.g),
		B: toByte(float64(c.B) / 255 * t.b),
		A: toByte(float64(c.A) / 255 * t.a),
	}
}

func toByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}

	if v >= 1 {
		return 255
	}

	return uint8(v*255 + 0.5)
}

// Package texturegen renders the boot splash's particle sprites (a soft
// round mote and a small hot ember) to PNG bytes. It is a build-time tool,
// not a runtime dependency of internal/enginesplash: its cmd/gen program
// writes the result once to internal/enginesplash/assets, which the splash
// then embeds directly. Regenerate with:
//
//	go run ./internal/enginesplash/texturegen/gen
package texturegen

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
)

const size = 32

// Mote renders a soft round point of light — solid core out to 13% of the
// sprite, then a smooth quadratic shoulder out to a 46% halo, tinted from
// white toward a cool moonlit edge. Used by the starfield and the Crux
// constellation.
func Mote() []byte {
	return encodePNG(render(size, 0.13, 0.46, func(distance, halo float64) color.NRGBA {
		warmth := clamp01(distance / halo)
		return color.NRGBA{
			R: 255,
			G: uint8(math.Round(250 - 16*warmth)),
			B: uint8(math.Round(230 - 44*warmth)),
		}
	}, func(t float64) float64 {
		inv := 1 - t
		return inv * inv
	}))
}

// Ember renders a small, hard, blindingly hot core with almost no halo
// (10%/34%, vs the mote's 13%/46%), cubic falloff so the edge stays defined
// instead of dissolving into glow, and a blackbody radial tint (white-hot
// core to yellow to orange rim). Used by the ring spark thrown off the
// compiled animation's orbit.
func Ember() []byte {
	return encodePNG(render(size, 0.10, 0.34, func(distance, edge float64) color.NRGBA {
		heat := clamp01(distance / edge)
		return color.NRGBA{
			R: 255,
			G: uint8(math.Round(252 - 96*heat)),
			B: uint8(math.Round(226 - 186*heat)),
		}
	}, func(t float64) float64 {
		inv := 1 - t
		return inv * inv * inv
	}))
}

// render rasterizes a radially symmetric particle sprite at 4x
// supersampling then downscales it, so the core/halo edges stay smooth at
// the small sizes particles actually draw at. core and halo are fractions
// of the sprite's size; tint returns the sprite's RGB at a given distance
// (already clamped to halo) — alpha comes from shoulder(t), t=0 at core's
// edge and t=1 at halo's edge.
func render(edge int, core, halo float64, tint func(distance, halo float64) color.NRGBA, shoulder func(t float64) float64) *image.NRGBA {
	const supersample = 4
	highSize := edge * supersample
	high := image.NewNRGBA(image.Rect(0, 0, highSize, highSize))
	center := float64(highSize-1) * 0.5
	coreR := float64(highSize) * core
	haloR := float64(highSize) * halo

	for y := 0; y < highSize; y++ {
		for x := 0; x < highSize; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			distance := math.Hypot(dx, dy)

			alpha := 0.0
			switch {
			case distance <= coreR:
				alpha = 1
			case distance <= haloR:
				alpha = shoulder((distance - coreR) / (haloR - coreR))
			}

			c := tint(distance, haloR)
			c.A = uint8(math.Round(clamp01(alpha) * 255))
			high.SetNRGBA(x, y, c)
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, edge, edge))
	xdraw.CatmullRom.Scale(img, img.Bounds(), high, high.Bounds(), stddraw.Src, nil)
	return img
}

func clamp01(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // deterministic in-memory input; encoding cannot fail
	}
	return buf.Bytes()
}

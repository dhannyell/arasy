package raster

import (
	"image"
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/particle"
)

func TestBlitScaledModeAdditiveSaturatesEveryChannel(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 1, 1))
	dst.SetRGBA(0, 0, color.RGBA{R: 250, G: 5, B: 10, A: 240})

	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 250, A: 20})

	blitScaledMode(
		dst,
		src,
		src.Bounds(),
		0,
		0,
		1,
		1,
		dst.Bounds(),
		noTint,
		particle.BlendAdditive,
	)

	want := color.RGBA{R: 255, G: 25, B: 255, A: 255}
	if got := dst.RGBAAt(0, 0); got != want {
		t.Fatalf("additive pixel = %#v, want %#v", got, want)
	}
}

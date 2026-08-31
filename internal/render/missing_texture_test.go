package render_test

import (
	"image/color"
	"testing"

	internalrender "github.com/dhannyell/arasy/internal/render"
)

func TestMissingTextureRGBAKeepsDiagnosticPatternVisible(t *testing.T) {
	placeholder := internalrender.MissingTextureRGBA()
	if got := placeholder.Bounds().Dx(); got != internalrender.MissingTextureSize {
		t.Fatalf("width = %d, want %d", got, internalrender.MissingTextureSize)
	}
	if got := placeholder.RGBAAt(0, 0); got != (color.RGBA{R: 255, B: 255, A: 255}) {
		t.Fatalf("corner = %v, want magenta background", got)
	}
	center := internalrender.MissingTextureSize / 2
	if got := placeholder.RGBAAt(center, center); got != (color.RGBA{A: 255}) {
		t.Fatalf("center = %v, want black diagnostic mark", got)
	}
}

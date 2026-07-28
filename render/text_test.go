package render_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestMeasureTextUsesCanonicalMonospaceMetrics(t *testing.T) {
	got := render.MeasureText("Gols", 20)

	if got.X != 48 {
		t.Fatalf("MeasureText width = %v, want 48", got.X)
	}
	if got.Y != 25 {
		t.Fatalf("MeasureText height = %v, want 25", got.Y)
	}
}

func TestMeasureTextCountsRunes(t *testing.T) {
	got := render.MeasureText("\u00e19", 10)

	if got.X != 12 {
		t.Fatalf("MeasureText width = %v, want 12", got.X)
	}
}

func TestMeasureTextWithNonPositiveSizeIsZero(t *testing.T) {
	got := render.MeasureText("text", 0)

	if got.X != 0 || got.Y != 0 {
		t.Fatalf("MeasureText = %v, want zero vector", got)
	}
}

func TestMeasureTextSpacedAddsInterRuneTracking(t *testing.T) {
	got := render.MeasureTextSpaced("ABC", 10, 2)
	if got.X != 22 {
		t.Fatalf("width = %v, want 22", got.X)
	}
	got = render.MeasureTextSpaced("ABC", 10, -2)
	if got.X != 14 {
		t.Fatalf("negative tracking width = %v, want 14", got.X)
	}
}

func TestMeasureTextMetricsCachesSizeNaturalWidthAndRuneCount(t *testing.T) {
	got := render.MeasureTextMetrics("áBC", 10, 2)
	if got.RuneCount != 3 {
		t.Fatalf("RuneCount = %d, want 3", got.RuneCount)
	}
	if got.NaturalWidth != 18 {
		t.Fatalf("NaturalWidth = %v, want 18", got.NaturalWidth)
	}
	if got.Size != (math2d.Vec2{X: 22, Y: 12.5}) {
		t.Fatalf("Size = %+v, want (22, 12.5)", got.Size)
	}
}

package render_test

import (
	"testing"

	"github.com/dhannyell/frameforge/render"
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

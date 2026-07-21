package renderbatch_test

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func assertNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %v, want %v ± %v", name, got, want, tolerance)
	}
}

func assertRectNear(t *testing.T, got, want math2d.Rect, tolerance float64) {
	t.Helper()
	assertNear(t, "Min.X", got.Min.X, want.Min.X, tolerance)
	assertNear(t, "Min.Y", got.Min.Y, want.Min.Y, tolerance)
	assertNear(t, "Max.X", got.Max.X, want.Max.X, tolerance)
	assertNear(t, "Max.Y", got.Max.Y, want.Max.Y, tolerance)
}

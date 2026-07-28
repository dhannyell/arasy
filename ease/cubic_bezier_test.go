package ease_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/ease"
)

func TestCubicBezierEndpointsAreExact(t *testing.T) {
	curves := []ease.Curve{
		ease.MustCubicBezier(0.25, 0.1, 0.25, 1),
		ease.MustCubicBezier(0.42, 0, 1, 1),
		ease.MustCubicBezier(0, 0, 0.58, 1),
		ease.MustCubicBezier(0.42, 0, 0.58, 1),
	}
	for _, curve := range curves {
		if got := curve.Sample(0); got != 0 {
			t.Fatalf("Sample(0) = %v, want 0", got)
		}
		if got := curve.Sample(1); got != 1 {
			t.Fatalf("Sample(1) = %v, want 1", got)
		}
	}
}

func TestCubicBezierCanonicalCSSCurves(t *testing.T) {
	tests := []struct {
		name string
		x1   float64
		y1   float64
		x2   float64
		y2   float64
		want float64
	}{
		{name: "ease", x1: 0.25, y1: 0.1, x2: 0.25, y2: 1, want: 0.802403},
		{name: "ease-in", x1: 0.42, y1: 0, x2: 1, y2: 1, want: 0.315357},
		{name: "ease-out", x1: 0, y1: 0, x2: 0.58, y2: 1, want: 0.684643},
		{name: "ease-in-out", x1: 0.42, y1: 0, x2: 0.58, y2: 1, want: 0.5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ease.MustCubicBezier(test.x1, test.y1, test.x2, test.y2).Sample(0.5)
			if math.Abs(got-test.want) > 1e-5 {
				t.Fatalf("Sample(0.5) = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCubicBezierRejectsInvalidCoordinates(t *testing.T) {
	values := [][4]float64{
		{-0.01, 0, 1, 1},
		{0, 0, 1.01, 1},
		{math.NaN(), 0, 1, 1},
		{0, math.Inf(1), 1, 1},
	}
	for _, value := range values {
		if _, err := ease.NewCubicBezier(value[0], value[1], value[2], value[3]); err == nil {
			t.Fatalf("NewCubicBezier%v returned nil error", value)
		}
	}
}

func TestCubicBezierPreservesYOvershoot(t *testing.T) {
	curve := ease.MustCubicBezier(0.2, 2, 0.8, 2)
	if got := curve.Sample(0.5); got <= 1 {
		t.Fatalf("Sample(0.5) = %v, want overshoot above 1", got)
	}
}

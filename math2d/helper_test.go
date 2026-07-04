package math2d_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func TestLerp(t *testing.T) {
	scenarios := []struct {
		name string
		a    float64
		b    float64
		t    float64
		want float64
	}{
		{"Start", 0.0, 100.0, 0.0, 0.0},
		{"Middle", 0.0, 100.0, 0.5, 50.0},
		{"End", 0.0, 100.0, 1.0, 100.0},
		{"Fract", 0.0, 10.0, 0.25, 2.5},
		{"Neg", -50.0, 50.0, 0.5, 0.0},
		{"Only Neg", -20.0, -10.0, 0.2, -18.0},
		{"Inversion", 10.0, 0.0, 0.3, 7.0},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := math2d.Lerp(s.a, s.b, s.t)

			if got != s.want {
				t.Fatalf("got %v, want %v", got, s.want)
			}
		})
	}
}

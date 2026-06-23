package render_test

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/render"
)

func TestColorFromHex(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want render.Color
	}{
		{
			name: "six digits with hash",
			hex:  "#FF5733",
			want: render.Color{R: 1, G: 87.0 / 255.0, B: 51.0 / 255.0, A: 1},
		},
		{
			name: "eight digits with alpha",
			hex:  "#FF5733CC",
			want: render.Color{R: 1, G: 87.0 / 255.0, B: 51.0 / 255.0, A: 204.0 / 255.0},
		},
		{
			name: "three digits",
			hex:  "F00",
			want: render.Color{R: 1, G: 0, B: 0, A: 1},
		},
		{
			name: "four digits with alpha",
			hex:  "#00F8",
			want: render.Color{R: 0, G: 0, B: 1, A: 136.0 / 255.0},
		},
		{
			name: "trims whitespace",
			hex:  "  #000000  ",
			want: render.Color{R: 0, G: 0, B: 0, A: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render.ColorFromHex(tt.hex)
			if err != nil {
				t.Fatalf("ColorFromHex failed: %v", err)
			}

			assertColorNear(t, got, tt.want)
		})
	}
}

func TestColorFromHexInvalid(t *testing.T) {
	tests := []string{
		"",
		"#",
		"#FF",
		"#FFFFF",
		"#GG0000",
		"#FF00000X",
	}

	for _, hex := range tests {
		t.Run(hex, func(t *testing.T) {
			_, err := render.ColorFromHex(hex)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func assertColorNear(t *testing.T, got, want render.Color) {
	t.Helper()

	if !near(got.R, want.R) ||
		!near(got.G, want.G) ||
		!near(got.B, want.B) ||
		!near(got.A, want.A) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func near(a, b float64) bool {
	const epsilon = 0.000001
	return math.Abs(a-b) <= epsilon
}

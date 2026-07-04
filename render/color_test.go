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

func TestRGBA8(t *testing.T) {
	got := render.RGBA8(255, 128, 0, 255)

	want := render.Color{R: 1, G: 0.5019607843137255, B: 0, A: 1}

	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestRGBA(t *testing.T) {
	got := render.RGBA(0.1, 0.2, 0.3, 0.4)

	want := render.Color{R: 0.1, G: 0.2, B: 0.3, A: 0.4}

	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestColorLerp(t *testing.T) {
	scenarios := []struct {
		name   string
		c1, c2 render.Color
		t      float64
		want   render.Color
	}{
		{
			name: "Full Black",
			c1:   render.Color{A: 1},
			c2:   render.Color{R: 1, G: 1, B: 1, A: 1},
			t:    0.0,
			want: render.Color{A: 1},
		},
		{
			name: "Half Black",
			c1:   render.Color{A: 1},
			c2:   render.Color{R: 1, G: 1, B: 1, A: 1},
			t:    0.5,
			want: render.Color{R: 0.5, G: 0.5, B: 0.5, A: 1.0},
		},
		{
			name: "Full White",
			c1:   render.Color{A: 1},
			c2:   render.Color{R: 1, G: 1, B: 1, A: 1},
			t:    1.0,
			want: render.Color{R: 1.0, G: 1.0, B: 1.0, A: 1.0},
		},
		{
			name: "Transition One Channel",
			c1:   render.Color{A: 1},
			c2:   render.Color{R: 1, G: 0, B: 0, A: 1},
			t:    0.25,
			want: render.Color{R: 0.25, G: 0.0, B: 0.0, A: 1.0},
		},
		{
			name: "Fade Out",
			c1:   render.Color{R: 1, A: 1},
			c2:   render.Color{R: 1},
			t:    0.75,
			want: render.Color{R: 1.0, G: 0.0, B: 0.0, A: 0.25},
		},
		{
			name: "Invert",
			c1:   render.Color{R: 1, A: 1},
			c2:   render.Color{G: 1, A: 1},
			t:    0.5,
			want: render.Color{R: 0.5, G: 0.5, B: 0.0, A: 1.0},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := s.c1.Lerp(s.c2, s.t)

			if got != s.want {
				t.Fatalf("got: %v, want: %v", got, s.want)
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

func nearScalar(a, b float64) bool {
	const epsilon = 0.001
	return math.Abs(a-b) <= epsilon
}

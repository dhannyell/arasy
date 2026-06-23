package math2d_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func TestNewRect(t *testing.T) {
	got := math2d.NewRect(
		math2d.Vec2{X: 10, Y: 20},
		math2d.Vec2{X: 30, Y: 40},
	)

	want := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRectWidth(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	got := rect.Width()
	want := 30.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRectHeight(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	got := rect.Height()
	want := 40.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRectSize(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	got := rect.Size()
	want := math2d.Vec2{X: 30, Y: 40}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRectCenter(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	got := rect.Center()
	want := math2d.Vec2{X: 25, Y: 40}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRectContains(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	tests := []struct {
		name  string
		point math2d.Vec2
		want  bool
	}{
		{
			name:  "inside",
			point: math2d.Vec2{X: 20, Y: 30},
			want:  true,
		},
		{
			name:  "on min edge",
			point: math2d.Vec2{X: 10, Y: 20},
			want:  true,
		},
		{
			name:  "on max x edge",
			point: math2d.Vec2{X: 40, Y: 30},
			want:  false,
		},
		{
			name:  "on max y edge",
			point: math2d.Vec2{X: 20, Y: 60},
			want:  false,
		},
		{
			name:  "on max corner",
			point: math2d.Vec2{X: 40, Y: 60},
			want:  false,
		},
		{
			name:  "left outside",
			point: math2d.Vec2{X: 9, Y: 30},
			want:  false,
		},
		{
			name:  "right outside",
			point: math2d.Vec2{X: 41, Y: 30},
			want:  false,
		},
		{
			name:  "above outside",
			point: math2d.Vec2{X: 20, Y: 19},
			want:  false,
		},
		{
			name:  "below outside",
			point: math2d.Vec2{X: 20, Y: 61},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rect.Contains(tt.point)

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectOverlaps(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	tests := []struct {
		name  string
		other math2d.Rect
		want  bool
	}{
		{
			name: "overlapping",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 30, Y: 50},
				Max: math2d.Vec2{X: 50, Y: 80},
			},
			want: true,
		},
		{
			name: "inside",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 15, Y: 25},
				Max: math2d.Vec2{X: 20, Y: 30},
			},
			want: true,
		},
		{
			name: "touching edge",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 40, Y: 30},
				Max: math2d.Vec2{X: 50, Y: 40},
			},
			want: false,
		},
		{
			name: "left outside",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 0, Y: 30},
				Max: math2d.Vec2{X: 9, Y: 40},
			},
			want: false,
		},
		{
			name: "right outside",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 41, Y: 30},
				Max: math2d.Vec2{X: 50, Y: 40},
			},
			want: false,
		},
		{
			name: "above outside",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 20, Y: 0},
				Max: math2d.Vec2{X: 30, Y: 19},
			},
			want: false,
		},
		{
			name: "below outside",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 20, Y: 61},
				Max: math2d.Vec2{X: 30, Y: 70},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rect.Overlaps(tt.other)

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectTouchesOrOverlaps(t *testing.T) {
	rect := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 40, Y: 60},
	}

	tests := []struct {
		name  string
		other math2d.Rect
		want  bool
	}{
		{
			name: "overlapping",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 30, Y: 50},
				Max: math2d.Vec2{X: 50, Y: 80},
			},
			want: true,
		},
		{
			name: "touching edge",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 40, Y: 30},
				Max: math2d.Vec2{X: 50, Y: 40},
			},
			want: true,
		},
		{
			name: "separated",
			other: math2d.Rect{
				Min: math2d.Vec2{X: 41, Y: 30},
				Max: math2d.Vec2{X: 50, Y: 40},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rect.TouchesOrOverlaps(tt.other)

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewRectFromCorners(t *testing.T) {
	got := math2d.NewRectFromCorners(
		math2d.Vec2{X: 10, Y: 20},
		math2d.Vec2{X: 30, Y: 40},
	)

	got.Width()

	want := math2d.Rect{
		Min: math2d.Vec2{X: 10, Y: 20},
		Max: math2d.Vec2{X: 30, Y: 40},
	}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

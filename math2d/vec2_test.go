package math2d_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestVec2Add(t *testing.T) {
	got := math2d.Vec2{X: 1, Y: 2}.Add(math2d.Vec2{X: 3, Y: 4})
	want := math2d.Vec2{X: 4, Y: 6}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Sub(t *testing.T) {
	got := math2d.Vec2{X: 5, Y: 7}.Sub(math2d.Vec2{X: 2, Y: 3})
	want := math2d.Vec2{X: 3, Y: 4}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Mul(t *testing.T) {
	got := math2d.Vec2{X: 2, Y: -3}.Mul(4)
	want := math2d.Vec2{X: 8, Y: -12}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Div(t *testing.T) {
	got := math2d.Vec2{X: 8, Y: -12}.Div(4)
	want := math2d.Vec2{X: 2, Y: -3}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Dot(t *testing.T) {
	got := math2d.Vec2{X: 1, Y: 3}.Dot(math2d.Vec2{X: 4, Y: -2})
	want := -2.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2LenSq(t *testing.T) {
	got := math2d.Vec2{X: 3, Y: 4}.LenSq()
	want := 25.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Len(t *testing.T) {
	got := math2d.Vec2{X: 3, Y: 4}.Len()
	want := 5.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Normalize(t *testing.T) {
	got := math2d.Vec2{X: 3, Y: 4}.Normalize()
	want := math2d.Vec2{X: 0.6, Y: 0.8}

	assertVec2Near(t, got, want)
}

func TestVec2NormalizeZero(t *testing.T) {
	got := (math2d.Vec2{}).Normalize()
	want := math2d.Vec2{}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Distance(t *testing.T) {
	got := math2d.Vec2{X: 1, Y: 1}.Distance(math2d.Vec2{X: 4, Y: 5})
	want := 5.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2DistanceSq(t *testing.T) {
	got := math2d.Vec2{X: 1, Y: 1}.DistanceSq(math2d.Vec2{X: 4, Y: 5})
	want := 25.0

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVec2Lerp(t *testing.T) {
	got := math2d.Vec2{X: 0, Y: 10}.Lerp(math2d.Vec2{X: 10, Y: 20}, 0.25)
	want := math2d.Vec2{X: 2.5, Y: 12.5}

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertVec2Near(t *testing.T, got, want math2d.Vec2) {
	t.Helper()

	if !near(got.X, want.X) || !near(got.Y, want.Y) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func near(a, b float64) bool {
	const epsilon = 0.000001
	return math.Abs(a-b) <= epsilon
}

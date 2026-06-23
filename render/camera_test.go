package render_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestCameraWorldToScreen(t *testing.T) {
	camera := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: 100, Y: 50},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: 1000, Y: 500},
		),
	}

	tests := []struct {
		name  string
		point math2d.Vec2
		want  math2d.Vec2
	}{
		{
			name:  "world min maps to viewport min",
			point: math2d.Vec2{X: 0, Y: 0},
			want:  math2d.Vec2{X: 0, Y: 0},
		},
		{
			name:  "world center maps to viewport center",
			point: math2d.Vec2{X: 50, Y: 25},
			want:  math2d.Vec2{X: 500, Y: 250},
		},
		{
			name:  "world max maps to viewport max",
			point: math2d.Vec2{X: 100, Y: 50},
			want:  math2d.Vec2{X: 1000, Y: 500},
		},
		{
			name:  "arbitrary point scales proportionally",
			point: math2d.Vec2{X: 25, Y: 10},
			want:  math2d.Vec2{X: 250, Y: 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := camera.WorldToScreen(tt.point)

			assertVec2Near(t, got, tt.want)
		})
	}
}

func TestCameraScreenToWorld(t *testing.T) {
	camera := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: 100, Y: 50},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: 1000, Y: 500},
		),
	}

	tests := []struct {
		name  string
		point math2d.Vec2
		want  math2d.Vec2
	}{
		{
			name:  "viewport min maps to world min",
			point: math2d.Vec2{X: 0, Y: 0},
			want:  math2d.Vec2{X: 0, Y: 0},
		},
		{
			name:  "viewport center maps to world center",
			point: math2d.Vec2{X: 500, Y: 250},
			want:  math2d.Vec2{X: 50, Y: 25},
		},
		{
			name:  "viewport max maps to world max",
			point: math2d.Vec2{X: 1000, Y: 500},
			want:  math2d.Vec2{X: 100, Y: 50},
		},
		{
			name:  "arbitrary point scales proportionally",
			point: math2d.Vec2{X: 250, Y: 100},
			want:  math2d.Vec2{X: 25, Y: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := camera.ScreenToWorld(tt.point)

			assertVec2Near(t, got, tt.want)
		})
	}
}

func TestCameraWithOffsetViewport(t *testing.T) {
	camera := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: -50, Y: -25},
			math2d.Vec2{X: 100, Y: 50},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{X: 100, Y: 200},
			math2d.Vec2{X: 1000, Y: 500},
		),
	}

	worldPoint := math2d.Vec2{X: 0, Y: 0}
	screenPoint := math2d.Vec2{X: 600, Y: 450}

	assertVec2Near(t, camera.WorldToScreen(worldPoint), screenPoint)
	assertVec2Near(t, camera.ScreenToWorld(screenPoint), worldPoint)
}

func TestCameraRoundTrip(t *testing.T) {
	camera := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: -10, Y: 5},
			math2d.Vec2{X: 80, Y: 40},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{X: 20, Y: 30},
			math2d.Vec2{X: 1280, Y: 720},
		),
	}

	points := []math2d.Vec2{
		{X: -10, Y: 5},
		{X: 0, Y: 10},
		{X: 30, Y: 25},
		{X: 70, Y: 45},
	}

	for _, point := range points {
		screen := camera.WorldToScreen(point)
		got := camera.ScreenToWorld(screen)

		assertVec2Near(t, got, point)
	}
}

func assertVec2Near(t *testing.T, got, want math2d.Vec2) {
	t.Helper()

	if !near(got.X, want.X) || !near(got.Y, want.Y) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

package math3d

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestCameraBasis(t *testing.T) {
	scenarios := []struct {
		name   string
		eye    Vec3
		target Vec3
		want   [3]Vec3
	}{
		{
			name:   "top-down",
			eye:    Vec3{0, 0, 10},
			target: Vec3{0, 0, 0},
			want: [3]Vec3{
				{1, 0, 0},  // right
				{0, 1, 0},  // up
				{0, 0, -1}, // forward
			},
		},
		{
			name:   "horizontal+Y",
			eye:    Vec3{0, 0, 0},
			target: Vec3{0, 10, 0},
			want: [3]Vec3{
				{1, 0, 0},
				{0, 0, 1},
				{0, 1, 0},
			},
		},
		{
			name:   "oblique",
			eye:    Vec3{0, -10, 10},
			target: Vec3{0, 0, 0},
			want: [3]Vec3{
				{1, 0, 0},
				{0, 0.7071067811865475, 0.7071067811865475},
				{0, 0.7071067811865475, -0.7071067811865475},
			},
		},
		{
			name:   "target equal eye",
			eye:    Vec3{5, 5, 5},
			target: Vec3{5, 5, 5},
			want:   [3]Vec3{{}, {}, {}},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			camera := Camera{
				Eye:    scenario.eye,
				Target: scenario.target,
			}
			right, up, forward := camera.basis()

			if scenario.want[0] != right {
				t.Errorf("basis right: got %v, want %v", right, scenario.want[0])
			}

			if scenario.want[1] != up {
				t.Errorf("basis up: got %v, want %v", up, scenario.want[1])
			}

			if scenario.want[2] != forward {
				t.Errorf("basis forward: got %v, want %v", forward, scenario.want[2])
			}
		})
	}
}

func TestCameraProjectPoint(t *testing.T) {
	camera := Camera{
		Eye:      Vec3{0, 0, 10},
		Target:   Vec3{0, 0, 0},
		FOV:      math.Pi / 2,
		Near:     0.1,
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600}),
	}
	scenarios := []struct {
		name       string
		point      Vec3
		wantScreen math2d.Vec2
		wantDepth  float64
		wantOK     bool
	}{
		{
			name:       "center point",
			point:      Vec3{0, 0, 0},
			wantScreen: math2d.Vec2{X: 400, Y: 300},
			wantDepth:  10,
			wantOK:     true,
		},
		{
			name:       "point outside axis",
			point:      Vec3{1, 2, 0},
			wantScreen: math2d.Vec2{X: 430.0000000000009, Y: 239.99999999999818},
			wantDepth:  10,
			wantOK:     true,
		},
		{
			name:       "point behind camera",
			point:      Vec3{0, 0, 20},
			wantScreen: math2d.Vec2{},
			wantDepth:  -10,
			wantOK:     false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			gotScreen, gotDepth, gotErr := camera.ProjectPoint(scenario.point)

			if gotScreen != scenario.wantScreen {
				t.Errorf("projection screen: got %v, want %v", gotScreen, scenario.wantScreen)
			}

			if gotDepth != scenario.wantDepth {
				t.Errorf("projection depth: got %v, want %v", gotDepth, scenario.wantDepth)
			}

			if gotErr != scenario.wantOK {
				t.Errorf("projection error: got %v, want %v", gotErr, scenario.wantOK)
			}
		})
	}
}

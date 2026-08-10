package render_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
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

func TestWorldScalarToScreen(t *testing.T) {
	camera := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: 1, Y: 10},
			math2d.Vec2{X: 105, Y: 68},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{X: 10, Y: 100},
			math2d.Vec2{X: 1920, Y: 1080},
		),
	}

	tests := []struct {
		name               string
		worldScalar        float64
		wantViewportScalar float64
	}{
		{
			name:               "center of the world",
			worldScalar:        52.5,
			wantViewportScalar: 833.823,
		},
		{
			name:               "init of penalty area",
			worldScalar:        88.5,
			wantViewportScalar: 1405.588,
		},
		{
			name:               "penalty spot",
			worldScalar:        94,
			wantViewportScalar: 1492.941,
		},
	}

	for _, tt := range tests {
		got := camera.WorldScalarToScreen(tt.worldScalar)

		assertScalarNear(t, got, tt.wantViewportScalar)
	}
}

func TestScreenScalarToWorld(t *testing.T) {
	camera := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: 1, Y: 10},
			math2d.Vec2{X: 105, Y: 68},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{X: 10, Y: 100},
			math2d.Vec2{X: 1920, Y: 1080},
		),
	}

	tests := []struct {
		name            string
		screenScalar    float64
		wantWorldScalar float64
	}{
		{
			name:            "center of the world",
			screenScalar:    833.82,
			wantWorldScalar: 52.500,
		},
		{
			name:            "init of penalty area",
			screenScalar:    1405.588,
			wantWorldScalar: 88.500,
		},
		{
			name:            "penalty spot",
			screenScalar:    1492.941,
			wantWorldScalar: 94.000,
		},
	}

	for _, tt := range tests {
		got := camera.ScreenScalarToWorld(tt.screenScalar)

		assertScalarNear(t, got, tt.wantWorldScalar)
	}
}

func TestCameraForLayerUsesFirstMatchingRange(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: -100, Y: -50}, math2d.Vec2{X: 200, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 400}),
		Parallax: []render.LayerParallax{
			{FromLayer: 1, ToLayer: 3, Factor: math2d.Vec2{X: 0.5, Y: 0.5}, Anchor: math2d.Vec2{X: 10, Y: -5}},
			{FromLayer: 3, ToLayer: 5, Factor: math2d.Vec2{X: 0, Y: 0}},
		},
	}

	if got := camera.ForLayer(0); !reflect.DeepEqual(got, camera) {
		t.Fatalf("unlisted layer changed camera: got %#v, want %#v", got, camera)
	}
	if got := camera.ForLayer(6); !reflect.DeepEqual(got, camera) {
		t.Fatalf("layer outside ranges changed camera: got %#v, want %#v", got, camera)
	}

	first := camera.ForLayer(1)
	wantFirst := camera
	wantFirst.World = math2d.NewRect(math2d.Vec2{X: -95, Y: -52.5}, math2d.Vec2{X: 200, Y: 100})
	if !reflect.DeepEqual(first, wantFirst) {
		t.Fatalf("first range result = %#v, want %#v", first, wantFirst)
	}

	if got := camera.ForLayer(3); !reflect.DeepEqual(got, first) {
		t.Fatalf("overlapping range did not keep first declaration: got %#v, want %#v", got, first)
	}

	factorOne := camera
	factorOne.Parallax = []render.LayerParallax{{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 1, Y: 1}}}
	if got := factorOne.ForLayer(0); !reflect.DeepEqual(got, factorOne) {
		t.Fatalf("factor one changed camera: got %#v, want %#v", got, factorOne)
	}
}

func TestCameraParallaxScrollFactors(t *testing.T) {
	base := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: -50, Y: -25}, math2d.Vec2{X: 100, Y: 50}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 400}),
		Parallax: []render.LayerParallax{{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 0, Y: 0}}},
	}
	point := math2d.Vec2{X: 12, Y: 4}
	baseScreen := base.ForLayer(0).WorldToScreen(point)

	for _, delta := range []math2d.Vec2{{}, {X: 20, Y: -10}, {X: -70, Y: 35}, {X: 145, Y: 80}, {X: -200, Y: -45}} {
		got := base.Pan(delta).ForLayer(0).WorldToScreen(point)
		assertVec2Near(t, got, baseScreen)
	}

	half := base
	half.Parallax = []render.LayerParallax{{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 0.5, Y: 0.5}}}
	start := half.ForLayer(0).WorldToScreen(point)
	shifted := half.Pan(math2d.Vec2{X: 20, Y: -10}).ForLayer(0).WorldToScreen(point)
	baseShifted := half.Pan(math2d.Vec2{X: 20, Y: -10}).WorldToScreen(point)
	startBase := half.WorldToScreen(point)

	assertVec2Near(t, shifted, math2d.Vec2{
		X: start.X + (baseShifted.X-startBase.X)*0.5,
		Y: start.Y + (baseShifted.Y-startBase.Y)*0.5,
	})
}

func TestCameraParallaxPreservesProjectionRoundTrip(t *testing.T) {
	camera := render.Camera{
		World:      math2d.NewRect(math2d.Vec2{X: -300, Y: -150}, math2d.Vec2{X: 600, Y: 300}),
		Viewport:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1200, Y: 800}),
		DepthScale: 0.5,
		View:       math2d.Affine{A: 20, C: -20, B: 10, D: 10},
		Parallax:   []render.LayerParallax{{FromLayer: 2, ToLayer: 2, Factor: math2d.Vec2{X: 0.5, Y: 0.75}, Anchor: math2d.Vec2{X: 2, Y: 3}}},
	}

	derived := camera.ForLayer(2)
	for _, point := range []math2d.Vec2{{}, {X: 3.5, Y: 6.25}, {X: -2, Y: 7}, {X: 16, Y: 8}} {
		got := derived.ScreenToWorld(derived.WorldToScreen(point))
		if got.Distance(point) > 1e-9 {
			t.Fatalf("round trip moved %v to %v", point, got)
		}
	}
}

func TestCameraPanAndZoomAt(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 50}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 400}),
		Parallax: []render.LayerParallax{{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 0, Y: 0}}},
	}

	pan := camera.Pan(math2d.Vec2{X: 10, Y: -4})
	if got, want := pan.World, math2d.NewRect(math2d.Vec2{X: 10, Y: -4}, math2d.Vec2{X: 100, Y: 50}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Pan world = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(pan.Parallax, camera.Parallax) {
		t.Fatal("Pan changed parallax declarations")
	}

	anchor := math2d.Vec2{X: 25, Y: 10}
	before := camera.WorldToScreen(anchor)
	zoomed := camera.ZoomAt(2, anchor)
	assertVec2Near(t, zoomed.WorldToScreen(anchor), before)
	assertVec2Near(t, zoomed.World.Size(), math2d.Vec2{X: 50, Y: 25})

	restored := zoomed.ZoomAt(0.5, anchor)
	if !reflect.DeepEqual(restored.World, camera.World) {
		t.Fatalf("zoom round trip world = %#v, want %#v", restored.World, camera.World)
	}

	for _, factor := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if got := camera.ZoomAt(factor, anchor); !reflect.DeepEqual(got, camera) {
			t.Fatalf("invalid factor %v changed camera: got %#v, want %#v", factor, got, camera)
		}
	}
}

func TestWrapOffsetsCoversVisibleSpan(t *testing.T) {
	tests := []struct {
		name                   string
		visibleMin, visibleMax float64
		period                 float64
		wantFirst              float64
		wantCount              int
	}{
		{name: "span smaller than period", visibleMin: 0.25, visibleMax: 0.75, period: 1, wantFirst: 0, wantCount: 1},
		{name: "exact multiple", visibleMin: 0, visibleMax: 3, period: 1, wantFirst: 0, wantCount: 3},
		{name: "negative minimum", visibleMin: -1.25, visibleMax: 1.25, period: 1, wantFirst: -2, wantCount: 4},
		{name: "partial negative span", visibleMin: -0.2, visibleMax: 0.2, period: 1, wantFirst: -1, wantCount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, count := render.WrapOffsets(tt.visibleMin, tt.visibleMax, tt.period)
			if first != tt.wantFirst || count != tt.wantCount {
				t.Fatalf("WrapOffsets = (%v, %d), want (%v, %d)", first, count, tt.wantFirst, tt.wantCount)
			}
			if first+float64(count)*tt.period < tt.visibleMax {
				t.Fatalf("copies end at %v, before visible max %v", first+float64(count)*tt.period, tt.visibleMax)
			}
		})
	}
}

func assertVec2Near(t *testing.T, got, want math2d.Vec2) {
	t.Helper()

	if !near(got.X, want.X) || !near(got.Y, want.Y) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertScalarNear(t *testing.T, got, want float64) {
	t.Helper()

	if !nearScalar(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

package render_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func depthCamera(aspect render.AspectMode, depthScale float64) render.Camera {
	return render.Camera{
		World:      math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 100, Y: 50}),
		Viewport:   math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 800, Y: 600}),
		Aspect:     aspect,
		DepthScale: depthScale,
	}
}

// The numbers below were taken from the camera before DepthScale existed. They
// are here so a change to the depth path that leaks into the untilted camera
// fails loudly instead of quietly moving every existing game a few pixels.
func TestCameraZeroDepthScaleKeepsTheUntiltedMapping(t *testing.T) {
	tests := []struct {
		name   string
		aspect render.AspectMode
		world  math2d.Vec2
		screen math2d.Vec2
	}{
		{"stretch", render.AspectStretch, math2d.Vec2{X: 25, Y: 10}, math2d.Vec2{X: 200, Y: 120}},
		{"stretch far corner", render.AspectStretch, math2d.Vec2{X: 75, Y: 40}, math2d.Vec2{X: 600, Y: 480}},
		{"fit", render.AspectFit, math2d.Vec2{X: 25, Y: 10}, math2d.Vec2{X: 200, Y: 180}},
		{"expand", render.AspectExpand, math2d.Vec2{X: 25, Y: 10}, math2d.Vec2{X: 200, Y: 180}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			camera := depthCamera(tt.aspect, 0)

			assertVec2Near(t, camera.WorldToScreen(tt.world), tt.screen)
			assertVec2Near(t, camera.ScreenToWorld(tt.screen), tt.world)
		})
	}
}

func TestCameraDepthScaleRoundTrips(t *testing.T) {
	aspects := []struct {
		name string
		mode render.AspectMode
	}{
		{"stretch", render.AspectStretch},
		{"fit", render.AspectFit},
		{"expand", render.AspectExpand},
	}

	depthScales := []float64{1, 0.5, 0.01}

	points := []math2d.Vec2{
		{X: 0, Y: 0},
		{X: 50, Y: 25},
		{X: 12.5, Y: 47},
		{X: 100, Y: 50},
	}

	for _, aspect := range aspects {
		for _, depthScale := range depthScales {
			camera := depthCamera(aspect.mode, depthScale)

			for _, point := range points {
				got := camera.ScreenToWorld(camera.WorldToScreen(point))

				if !near(got.X, point.X) || !near(got.Y, point.Y) {
					t.Fatalf("%s at depth scale %v: round trip of %v gave %v", aspect.name, depthScale, point, got)
				}
			}
		}
	}
}

func TestCameraDepthScaleKeepsTheWorldCentreInPlace(t *testing.T) {
	centre := math2d.Vec2{X: 50, Y: 25}

	for _, aspect := range []render.AspectMode{render.AspectStretch, render.AspectFit, render.AspectExpand} {
		want := depthCamera(aspect, 1).WorldToScreen(centre)

		for _, depthScale := range []float64{0.75, 0.5, 0.01} {
			got := depthCamera(aspect, depthScale).WorldToScreen(centre)

			assertVec2Near(t, got, want)
		}
	}
}

func TestCameraDepthScaleShrinksDepthTowardsTheCentre(t *testing.T) {
	camera := depthCamera(render.AspectStretch, 0.5)
	centreY := camera.WorldToScreen(math2d.Vec2{X: 50, Y: 25}).Y

	upright := depthCamera(render.AspectStretch, 1)
	point := math2d.Vec2{X: 50, Y: 50}

	wantOffset := (upright.WorldToScreen(point).Y - centreY) * 0.5
	gotOffset := camera.WorldToScreen(point).Y - centreY

	if !near(gotOffset, wantOffset) {
		t.Fatalf("depth offset from centre: got %v, want %v", gotOffset, wantOffset)
	}

	if !near(camera.WorldToScreen(point).X, upright.WorldToScreen(point).X) {
		t.Fatal("foreshortening moved the horizontal axis")
	}
}

func TestCameraDepthScaleClampsOutOfRangeValues(t *testing.T) {
	point := math2d.Vec2{X: 30, Y: 45}
	upright := depthCamera(render.AspectStretch, 1).WorldToScreen(point)

	for _, depthScale := range []float64{2, 1000, math.NaN()} {
		got := depthCamera(render.AspectStretch, depthScale).WorldToScreen(point)

		assertVec2Near(t, got, upright)
	}

	// A negative or vanishing scale is nonsense rather than a request; it has to
	// clamp to the flattest usable view instead of producing Inf or NaN.
	for _, depthScale := range []float64{-1, 1e-12} {
		got := depthCamera(render.AspectStretch, depthScale).WorldToScreen(point)

		if math.IsNaN(got.Y) || math.IsInf(got.Y, 0) {
			t.Fatalf("depth scale %v produced %v", depthScale, got)
		}
	}
}

// A radius is shape, not scale: it has to foreshorten with the ground while a
// stroke width, which has no axis, must not.
func TestCameraForeshortensRadiiButNotScalars(t *testing.T) {
	radii := math2d.Vec2{X: 10, Y: 10}

	upright := depthCamera(render.AspectFit, 1)
	tilted := depthCamera(render.AspectFit, 0.5)

	if upright.Foreshortens() {
		t.Error("an untilted camera reports that it foreshortens")
	}
	if !tilted.Foreshortens() {
		t.Error("a tilted camera reports that it does not foreshorten")
	}

	flat := upright.WorldRadiiToScreen(radii)
	if !near(flat.X, flat.Y) {
		t.Fatalf("untilted radii are not round: %v", flat)
	}

	got := tilted.WorldRadiiToScreen(radii)
	if !near(got.X, flat.X) {
		t.Errorf("tilt changed the horizontal radius: %v, want %v", got.X, flat.X)
	}
	if !near(got.Y, flat.Y*0.5) {
		t.Errorf("vertical radius = %v, want %v", got.Y, flat.Y*0.5)
	}
}

func TestCameraScalarConversionsIgnoreDepthScale(t *testing.T) {
	for _, aspect := range []render.AspectMode{render.AspectStretch, render.AspectFit, render.AspectExpand} {
		want := depthCamera(aspect, 1).WorldScalarToScreen(10)

		for _, depthScale := range []float64{0.5, 0.01} {
			camera := depthCamera(aspect, depthScale)

			assertScalarNear(t, camera.WorldScalarToScreen(10), want)
			assertScalarNear(t, camera.ScreenScalarToWorld(want), 10)
		}
	}
}

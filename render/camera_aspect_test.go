package render_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestCameraAspectFitUsesCenteredViewport(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 100}),
		Aspect:   render.AspectFit,
	}

	resolved := camera.Resolve()

	assertRectNear(t, resolved.World, camera.World)
	assertRectNear(t, resolved.Viewport, math2d.NewRect(
		math2d.Vec2{X: 50, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	))
	if resolved.ScaleX != 1 || resolved.ScaleY != 1 {
		t.Fatalf("scale = (%v, %v), want (1, 1)", resolved.ScaleX, resolved.ScaleY)
	}
}

func TestCameraAspectExpandExpandsVisibleWorld(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 100}),
		Aspect:   render.AspectExpand,
	}

	resolved := camera.Resolve()

	assertRectNear(t, resolved.World, math2d.NewRect(
		math2d.Vec2{X: -50, Y: 0},
		math2d.Vec2{X: 200, Y: 100},
	))
	assertRectNear(t, resolved.Viewport, camera.Viewport)
	if resolved.ScaleX != 1 || resolved.ScaleY != 1 {
		t.Fatalf("scale = (%v, %v), want (1, 1)", resolved.ScaleX, resolved.ScaleY)
	}
}

func TestCameraResolveHandlesZeroWorldSize(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 100}),
		Aspect:   render.AspectFit,
	}

	resolved := camera.Resolve()

	assertRectNear(t, resolved.World, camera.World)
	assertRectNear(t, resolved.Viewport, camera.Viewport)
	if resolved.ScaleX != 0 || resolved.ScaleY != 0 {
		t.Fatalf("scale = (%v, %v), want (0, 0)", resolved.ScaleX, resolved.ScaleY)
	}
}

func assertRectNear(t *testing.T, got, want math2d.Rect) {
	t.Helper()

	assertVec2Near(t, got.Min, want.Min)
	assertVec2Near(t, got.Max, want.Max)
}

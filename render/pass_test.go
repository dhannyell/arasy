package render_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestPassResolveViewportUsesFullTargetForZeroViewport(t *testing.T) {
	bounds := math2d.NewRect(math2d.Vec2{X: 3, Y: 4}, math2d.Vec2{X: 80, Y: 40})
	pass := render.Pass{Camera: render.Camera{World: bounds}}

	resolved := pass.ResolveViewport(bounds)

	if resolved.Camera.Viewport != bounds {
		t.Fatalf("resolved viewport = %#v, want %#v", resolved.Camera.Viewport, bounds)
	}
}

func TestPassResolveViewportPreservesExplicitViewport(t *testing.T) {
	explicit := math2d.NewRect(math2d.Vec2{X: 2, Y: 5}, math2d.Vec2{X: 20, Y: 10})
	bounds := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 80, Y: 40})
	pass := render.Pass{Camera: render.Camera{Viewport: explicit}}

	if got := pass.ResolveViewport(bounds).Camera.Viewport; got != explicit {
		t.Fatalf("resolved viewport = %#v, want explicit %#v", got, explicit)
	}
}

func TestCameraForScreenPointUsesFrontmostExtraPassThenBase(t *testing.T) {
	base := render.Camera{Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})}
	first := render.Camera{Viewport: math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 40, Y: 40})}
	front := render.Camera{Viewport: math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 40, Y: 40})}
	passes := []render.Pass{{Camera: first}, {Camera: front}}

	if got := render.CameraForScreenPoint(base, passes, base.Viewport, math2d.Vec2{X: 25, Y: 25}); !reflect.DeepEqual(got, front) {
		t.Fatalf("overlap camera = %#v, want last declared pass", got)
	}
	if got := render.CameraForScreenPoint(base, passes, base.Viewport, math2d.Vec2{X: 90, Y: 90}); !reflect.DeepEqual(got, base) {
		t.Fatalf("fallback camera = %#v, want base", got)
	}
}

func TestCameraForScreenPointResolvesZeroViewport(t *testing.T) {
	base := render.Camera{Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})}
	passCamera := render.Camera{World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})}

	got := render.CameraForScreenPoint(base, []render.Pass{{Camera: passCamera}}, base.Viewport, math2d.Vec2{X: 50, Y: 50})
	if got.Viewport != base.Viewport || got.World != passCamera.World {
		t.Fatalf("resolved camera = %#v, want pass camera with base viewport", got)
	}

	base.Viewport = math2d.Rect{}
	got = render.CameraForScreenPoint(base, nil, math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 80, Y: 60}), math2d.Vec2{})
	if got.Viewport.Max != (math2d.Vec2{X: 80, Y: 60}) {
		t.Fatalf("base viewport = %#v, want resolved screen bounds", got.Viewport)
	}
}

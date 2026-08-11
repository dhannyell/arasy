package render_test

import (
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

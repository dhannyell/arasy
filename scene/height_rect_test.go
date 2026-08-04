package scene

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// A rect sprite carries its geometry in world coordinates and takes no
// position from its Transform, so it was the one sprite kind Height did
// nothing to: a raised platform drew its shadow on the ground and its body on
// the ground too, and the plane correction pivoted About a point outside it.
func TestHeightLiftsARectSprite(t *testing.T) {
	world := NewWorld()
	world.SetProjection(render.Projection{DepthScale: 0.5, LiftPerZ: 2})

	ground := math2d.NewRectFromCorners(
		math2d.Vec2{X: 9, Y: 9},
		math2d.Vec2{X: 11, Y: 11},
	)

	id := world.SpawnBundle(Bundle{
		Transform: &Transform{Position: math2d.Vec2{X: 10, Y: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		RectSprite: &RectSprite{
			Rect:       ground,
			BaseSprite: BaseSprite{Plane: PlaneGround},
		},
	})
	world.SetHeight(id, Height{Z: 3})

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}

	// Three units of height at two units of lift each, straight up the screen.
	if got := ground.Min.Y - rect.Rect.Min.Y; got != 6 {
		t.Fatalf("the platform rose %v, want 6", got)
	}

	if rect.Rect.Min.X != ground.Min.X || rect.Rect.Width() != ground.Width() {
		t.Fatalf("the lift moved the platform sideways or resized it: %v", rect.Rect)
	}
}

// The lift is a fact about height, not about the plane a sprite declares, and
// an upright sprite has to end up in the same place as a flat one.
func TestHeightLiftsAnUprightRectSpriteToo(t *testing.T) {
	world := NewWorld()
	world.SetProjection(render.Projection{DepthScale: 0.5, LiftPerZ: 2})

	id := world.SpawnBundle(Bundle{
		Transform: &Transform{Position: math2d.Vec2{X: 10, Y: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		RectSprite: &RectSprite{
			Rect: math2d.NewRectFromCorners(
				math2d.Vec2{X: 9, Y: 8},
				math2d.Vec2{X: 11, Y: 10},
			),
		},
	})
	world.SetHeight(id, Height{Z: 1})

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}

	if rect.Rect.Max.Y != 8 {
		t.Fatalf("the sprite's base sits at %v, want 8", rect.Rect.Max.Y)
	}

	// The plane correction pivots about the entity's own position, which is
	// where the lift left it -- so a standing sprite is stood back up on its
	// feet rather than about the middle of its art.
	anchor := math2d.Vec2{X: 10, Y: 8}
	if got := rect.Transform.Apply(anchor); got.Distance(anchor) > 1e-9 {
		t.Fatalf("the correction moved the sprite's anchor from %v to %v", anchor, got)
	}
}

// A sprite on the ground has nothing to lift, and must keep emitting no
// transform and untouched geometry.
func TestNoHeightLeavesARectSpriteAlone(t *testing.T) {
	world := NewWorld()
	world.SetProjection(render.Projection{DepthScale: 0.5, LiftPerZ: 2})

	ground := math2d.NewRectFromCorners(
		math2d.Vec2{X: 9, Y: 9},
		math2d.Vec2{X: 11, Y: 11},
	)

	world.SpawnBundle(Bundle{
		Transform: &Transform{Position: math2d.Vec2{X: 10, Y: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		RectSprite: &RectSprite{
			Rect:       ground,
			BaseSprite: BaseSprite{Plane: PlaneGround},
		},
	})

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}

	if rect.Rect != ground {
		t.Fatalf("geometry moved to %v", rect.Rect)
	}

	if !rect.Transform.IsIdentity() {
		t.Fatalf("a ground sprite picked up a transform: %v", rect.Transform)
	}
}

func firstRect(commands []render.Command) (render.DrawRect, bool) {
	for _, command := range commands {
		if rect, ok := command.(render.DrawRect); ok {
			return rect, true
		}
	}

	return render.DrawRect{}, false
}

package scene

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func spawnRect(world *World, pos, scale math2d.Vec2, local math2d.Rect) EntityID {
	return world.SpawnBundle(Bundle{
		Transform:  &Transform{Position: pos, Scale: scale},
		RectSprite: &RectSprite{Rect: local},
	})
}

var centeredUnit = math2d.NewRectFromCorners(
	math2d.Vec2{X: -1, Y: -1},
	math2d.Vec2{X: 1, Y: 1},
)

// The Transform positions the rect, like it already positions circles and
// images.
func TestRectSpriteIsAnchoredAtTheTransform(t *testing.T) {
	world := NewWorld()
	spawnRect(world, math2d.Vec2{X: 10, Y: 5}, math2d.Vec2{X: 1, Y: 1}, centeredUnit)

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}
	want := math2d.Vec2{X: 10, Y: 5}
	if got := rect.Rect.Center(); got != want {
		t.Fatalf("rect centered at %v, want %v", got, want)
	}
}

// Moving the Transform is enough; the sprite is never rewritten. This is the
// failure mode the old semantics allowed: a moved entity whose rect stayed
// behind, silently.
func TestMovingTheTransformMovesTheRect(t *testing.T) {
	world := NewWorld()
	id := spawnRect(world, math2d.Vec2{X: 10, Y: 5}, math2d.Vec2{X: 1, Y: 1}, centeredUnit)

	world.SetTransform(id, Transform{Position: math2d.Vec2{X: -3, Y: 7}, Scale: math2d.Vec2{X: 1, Y: 1}})

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}
	want := math2d.Vec2{X: -3, Y: 7}
	if got := rect.Rect.Center(); got != want {
		t.Fatalf("rect centered at %v, want %v", got, want)
	}
}

// Transform.Scale scales the shape about the anchor, so a doubled entity draws
// a doubled rect in place.
func TestTransformScaleScalesTheRect(t *testing.T) {
	world := NewWorld()
	spawnRect(world, math2d.Vec2{X: 10, Y: 5}, math2d.Vec2{X: 2, Y: 2}, centeredUnit)

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}
	if rect.Rect.Width() != 4 || rect.Rect.Height() != 4 {
		t.Fatalf("rect sized %vx%v, want 4x4", rect.Rect.Width(), rect.Rect.Height())
	}
	if got := rect.Rect.Center(); got != (math2d.Vec2{X: 10, Y: 5}) {
		t.Fatalf("scaling moved the anchor to %v", got)
	}
}

// An asymmetric local rect anchors away from its center -- a post whose
// Transform sits at its foot. This is what the anchored semantics buys over a
// plain centered size.
func TestAsymmetricRectAnchorsAtTheFoot(t *testing.T) {
	world := NewWorld()
	foot := math2d.Vec2{X: 10, Y: 20}
	spawnRect(world, foot, math2d.Vec2{X: 1, Y: 1}, math2d.NewRectFromCorners(
		math2d.Vec2{X: -2, Y: -14},
		math2d.Vec2{X: 2, Y: 0},
	))

	rect, ok := firstRect(world.RenderCommands())
	if !ok {
		t.Fatal("no rectangle was drawn")
	}
	if rect.Rect.Max.Y != foot.Y {
		t.Fatalf("the post's foot sits at %v, want %v", rect.Rect.Max.Y, foot.Y)
	}
	if rect.Rect.Min.Y != foot.Y-14 {
		t.Fatalf("the post's top sits at %v, want %v", rect.Rect.Min.Y, foot.Y-14)
	}
}

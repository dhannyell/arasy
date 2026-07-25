package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

func TestWorldCircleVsRectDetectsOverlapFromOutside(t *testing.T) {
	world := scene.NewWorld()

	// circle is the lower ID (spawned first) -> pair A
	circle := spawnCollidableCircle(world, math2d.Vec2{X: 7, Y: 0}, 2)
	rect := spawnCollidableRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})

	contacts := world.DetectCircleOverlapsGrid(8)
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}

	contact := contacts[0]
	if contact.A != circle || contact.B != rect {
		t.Fatalf("got pair (%d,%d), want (%d,%d)", contact.A, contact.B, circle, rect)
	}
	// closest point on rect is (8,0); delta circle->rect is +X
	wantNormal := math2d.Vec2{X: 1, Y: 0}
	if contact.Normal != wantNormal {
		t.Fatalf("got normal %+v, want %+v", contact.Normal, wantNormal)
	}
	if contact.Depth != 1 {
		t.Fatalf("got depth %v, want 1", contact.Depth)
	}
}

func TestWorldCircleVsRectMixedPairNormalIsAToBInBothIDOrders(t *testing.T) {
	// Same geometry, opposite spawn order. This is the regression guard for the
	// old invert-normal / A-B-ordering bug that teleported the ball.
	t.Run("CircleIsA", func(t *testing.T) {
		world := scene.NewWorld()
		circle := spawnCollidableCircle(world, math2d.Vec2{X: 7, Y: 0}, 2)
		rect := spawnCollidableRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})

		contacts := world.DetectCircleOverlapsGrid(8)
		if len(contacts) != 1 {
			t.Fatalf("got %d contacts, want 1", len(contacts))
		}
		contact := contacts[0]
		if contact.A != circle || contact.B != rect {
			t.Fatalf("got pair (%d,%d), want circle=%d < rect=%d", contact.A, contact.B, circle, rect)
		}
		// Normal must point A->B == circle->rect == +X
		if contact.Normal != (math2d.Vec2{X: 1, Y: 0}) {
			t.Fatalf("got normal %+v, want {1 0}", contact.Normal)
		}
	})

	t.Run("CircleIsB", func(t *testing.T) {
		world := scene.NewWorld()
		rect := spawnCollidableRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})
		circle := spawnCollidableCircle(world, math2d.Vec2{X: 7, Y: 0}, 2)

		contacts := world.DetectCircleOverlapsGrid(8)
		if len(contacts) != 1 {
			t.Fatalf("got %d contacts, want 1", len(contacts))
		}
		contact := contacts[0]
		if contact.A != rect || contact.B != circle {
			t.Fatalf("got pair (%d,%d), want rect=%d < circle=%d", contact.A, contact.B, rect, circle)
		}
		// Normal must point A->B == rect->circle == -X (flipped from above)
		if contact.Normal != (math2d.Vec2{X: -1, Y: 0}) {
			t.Fatalf("got normal %+v, want {-1 0}", contact.Normal)
		}
		if contact.Depth != 1 {
			t.Fatalf("got depth %v, want 1", contact.Depth)
		}
	})
}

func TestWorldCircleVsRectDoesNotDetectTouchingBorder(t *testing.T) {
	world := scene.NewWorld()

	// closest point is (8,0), distance == radius -> touching only, no contact
	spawnCollidableCircle(world, math2d.Vec2{X: 6, Y: 0}, 2)
	spawnCollidableRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})

	contacts := world.DetectCircleOverlapsGrid(8)
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts, want 0", len(contacts))
	}
}

func TestWorldCircleVsRectHandlesCenterInsideRect(t *testing.T) {
	world := scene.NewWorld()

	// circle center at (11,0) is inside rect x[8,12] y[-2,2]; nearest face is
	// the right one (distance 1), so it is pushed out along -X.
	circle := spawnCollidableCircle(world, math2d.Vec2{X: 11, Y: 0}, 2)
	rect := spawnCollidableRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})

	contacts := world.DetectCircleOverlapsGrid(8)
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}
	contact := contacts[0]
	if contact.A != circle || contact.B != rect {
		t.Fatalf("got pair (%d,%d), want (%d,%d)", contact.A, contact.B, circle, rect)
	}
	if contact.Normal != (math2d.Vec2{X: -1, Y: 0}) {
		t.Fatalf("got normal %+v, want {-1 0}", contact.Normal)
	}
	// depth = radius + distance to nearest face = 2 + 1
	if contact.Depth != 3 {
		t.Fatalf("got depth %v, want 3", contact.Depth)
	}
}

func TestWorldCircleVsRectResolvesBallBouncingOffStaticRect(t *testing.T) {
	// The breakout scenario: static rect (brick) spawned first (lower ID), a
	// dynamic circle (ball) moving into it. It must be pushed out cleanly and
	// have its normal velocity reflected â€” no teleport.
	world := scene.NewWorld()

	rect := spawnStaticRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})
	ball := world.Spawn()
	world.SetTransform(ball, scene.Transform{Position: math2d.Vec2{X: 7, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleCollider(ball, scene.CircleCollider{Radius: 2})
	world.SetBody(ball, scene.Body{InvMass: 1, Restitution: 1})
	world.SetVelocity(ball, scene.Velocity{Linear: math2d.Vec2{X: 5, Y: 0}})

	world.ResolveCircleOverlapsGrid(8)

	rectTransform, _ := world.Transform(rect)
	if rectTransform.Position != (math2d.Vec2{X: 10, Y: 0}) {
		t.Fatalf("static rect moved to %v, want {10 0}", rectTransform.Position)
	}

	ballTransform, _ := world.Transform(ball)
	// pushed out to the left face: closest was (8,0), depth 1, ball moves -X
	if ballTransform.Position != (math2d.Vec2{X: 6, Y: 0}) {
		t.Fatalf("ball position %v, want {6 0}", ballTransform.Position)
	}

	ballVelocity, _ := world.Velocity(ball)
	if ballVelocity.Linear != (math2d.Vec2{X: -5, Y: 0}) {
		t.Fatalf("ball velocity %v, want {-5 0}", ballVelocity.Linear)
	}
}

func TestWorldRectVsRectDetectsOverlapWithMinPenetrationNormal(t *testing.T) {
	world := scene.NewWorld()

	// A x[-5,5] y[-2,2], B x[-5,5] y[1,5]: overlapX=10, overlapY=1 -> Y axis
	a := spawnCollidableRect(world, math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 5, Y: 2})
	b := spawnCollidableRect(world, math2d.Vec2{X: 0, Y: 3}, math2d.Vec2{X: 5, Y: 2})

	contacts := world.DetectCircleOverlapsGrid(16)
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}
	contact := contacts[0]
	if contact.A != a || contact.B != b {
		t.Fatalf("got pair (%d,%d), want (%d,%d)", contact.A, contact.B, a, b)
	}
	// B center is above A center -> normal +Y
	if contact.Normal != (math2d.Vec2{X: 0, Y: 1}) {
		t.Fatalf("got normal %+v, want {0 1}", contact.Normal)
	}
	if contact.Depth != 1 {
		t.Fatalf("got depth %v, want 1", contact.Depth)
	}
}

func TestWorldRectVsRectBreaksAxisTieOnX(t *testing.T) {
	world := scene.NewWorld()

	// equal overlap on both axes -> X axis wins (deterministic tie-break)
	a := spawnCollidableRect(world, math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 2, Y: 2})
	b := spawnCollidableRect(world, math2d.Vec2{X: 3, Y: 3}, math2d.Vec2{X: 2, Y: 2})

	contacts := world.DetectCircleOverlapsGrid(16)
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}
	contact := contacts[0]
	if contact.A != a || contact.B != b {
		t.Fatalf("got pair (%d,%d), want (%d,%d)", contact.A, contact.B, a, b)
	}
	if contact.Normal != (math2d.Vec2{X: 1, Y: 0}) {
		t.Fatalf("got normal %+v, want {1 0}", contact.Normal)
	}
	if contact.Depth != 1 {
		t.Fatalf("got depth %v, want 1", contact.Depth)
	}
}

func TestWorldRectVsRectDoesNotDetectTouchingRects(t *testing.T) {
	world := scene.NewWorld()

	// A x[-2,2], B x[2,6]: overlapX == 0 -> no contact
	spawnCollidableRect(world, math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 2, Y: 2})
	spawnCollidableRect(world, math2d.Vec2{X: 4, Y: 0}, math2d.Vec2{X: 2, Y: 2})

	contacts := world.DetectCircleOverlapsGrid(16)
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts, want 0", len(contacts))
	}
}

func TestWorldCircleColliderTakesPrecedenceOverRectOnSameEntity(t *testing.T) {
	world := scene.NewWorld()

	// entity with both colliders must behave as a circle (D1 precedence rule)
	both := world.Spawn()
	world.SetTransform(both, scene.Transform{Position: math2d.Vec2{X: 0, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleCollider(both, scene.CircleCollider{Radius: 2})
	world.SetRectCollider(both, scene.RectCollider{HalfExtends: math2d.Vec2{X: 2, Y: 2}})
	world.SetBody(both, scene.Body{InvMass: 1, Restitution: 1})

	circle := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	contacts := world.DetectCircleOverlapsGrid(8)
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}
	contact := contacts[0]
	if contact.A != both || contact.B != circle {
		t.Fatalf("got pair (%d,%d), want (%d,%d)", contact.A, contact.B, both, circle)
	}
	// circle-vs-circle at distance 3, radii 2+2 -> depth 1, normal +X
	if contact.Normal != (math2d.Vec2{X: 1, Y: 0}) {
		t.Fatalf("got normal %+v, want {1 0}", contact.Normal)
	}
	if contact.Depth != 1 {
		t.Fatalf("got depth %v, want 1", contact.Depth)
	}
}

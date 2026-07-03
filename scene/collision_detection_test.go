package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func TestWorldDetectCircleOverlapsDetectsOverlap(t *testing.T) {
	world := scene.NewWorld()

	a := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	b := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}

	contact := contacts[0]
	if contact.A != a {
		t.Fatalf("got A %d, want %d", contact.A, a)
	}

	if contact.B != b {
		t.Fatalf("got B %d, want %d", contact.B, b)
	}

	wantNormal := math2d.Vec2{X: 1, Y: 0}
	if contact.Normal != wantNormal {
		t.Fatalf("got normal %+v, want %+v", contact.Normal, wantNormal)
	}

	if contact.Depth != 1 {
		t.Fatalf("got depth %v, want 1", contact.Depth)
	}
}

func TestWorldDetectCircleOverlapsDoesNotDetectTouchingCircles(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	spawnCollidableCircle(world, math2d.Vec2{X: 4, Y: 0}, 2)

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts, want 0", len(contacts))
	}
}

func TestWorldDetectCircleOverlapsSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)

	entity := world.Spawn()
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 2,
	})

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts, want 0", len(contacts))
	}
}

func TestWorldDetectRectOverlapsSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableRect(world, math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 2, Y: 2})

	entity := world.Spawn()
	world.SetRectCollider(entity, scene.RectCollider{
		HalfExtends: math2d.Vec2{X: 2, Y: 2},
	})

	contacts := world.DetectCircleOverlapsGrid(8)
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts, want 0", len(contacts))
	}
}

func TestWorldDetectCircleVsRectSkipsRectWithoutTransform(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)

	entity := world.Spawn()
	world.SetRectCollider(entity, scene.RectCollider{
		HalfExtends: math2d.Vec2{X: 2, Y: 2},
	})

	contacts := world.DetectCircleOverlapsGrid(8)
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts, want 0", len(contacts))
	}
}

func TestWorldDetectCircleOverlapsHandlesSamePosition(t *testing.T) {
	world := scene.NewWorld()

	a := spawnCollidableCircle(world, math2d.Vec2{X: 5, Y: 5}, 2)
	b := spawnCollidableCircle(world, math2d.Vec2{X: 5, Y: 5}, 3)

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 1 {
		t.Fatalf("got %d contacts, want 1", len(contacts))
	}

	contact := contacts[0]
	if contact.A != a {
		t.Fatalf("got A %d, want %d", contact.A, a)
	}

	if contact.B != b {
		t.Fatalf("got B %d, want %d", contact.B, b)
	}

	wantNormal := math2d.Vec2{X: 1, Y: 0}
	if contact.Normal != wantNormal {
		t.Fatalf("got normal %+v, want %+v", contact.Normal, wantNormal)
	}

	if contact.Depth != 5 {
		t.Fatalf("got depth %v, want 5", contact.Depth)
	}
}

func TestWorldDetectCircleOverlapsReturnsContactsInEntityOrder(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)
	third := spawnCollidableCircle(world, math2d.Vec2{X: 6, Y: 0}, 5)

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 3 {
		t.Fatalf("got %d contacts, want 3", len(contacts))
	}

	wantPairs := []struct {
		a scene.EntityID
		b scene.EntityID
	}{
		{a: first, b: second},
		{a: first, b: third},
		{a: second, b: third},
	}

	for i, want := range wantPairs {
		if contacts[i].A != want.a || contacts[i].B != want.b {
			t.Fatalf(
				"contact %d got pair (%d, %d), want (%d, %d)",
				i,
				contacts[i].A,
				contacts[i].B,
				want.a,
				want.b,
			)
		}
	}
}

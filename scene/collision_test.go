package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func TestWorldBounceCirclesInsideBounces(t *testing.T) {
	bounds := math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	)

	tests := []struct {
		name         string
		position     math2d.Vec2
		velocity     math2d.Vec2
		wantPosition math2d.Vec2
		wantVelocity math2d.Vec2
	}{
		{
			name:         "Right Edge",
			position:     math2d.Vec2{X: 98, Y: 50},
			velocity:     math2d.Vec2{X: 10, Y: 0},
			wantPosition: math2d.Vec2{X: 98, Y: 50},
			wantVelocity: math2d.Vec2{X: -10, Y: 0},
		},
		{
			name:         "Left Edge",
			position:     math2d.Vec2{X: 2, Y: 50},
			velocity:     math2d.Vec2{X: -10, Y: 0},
			wantPosition: math2d.Vec2{X: 2, Y: 50},
			wantVelocity: math2d.Vec2{X: 10, Y: 0},
		},
		{
			name:         "Bottom Edge",
			position:     math2d.Vec2{X: 50, Y: 98},
			velocity:     math2d.Vec2{X: 0, Y: 10},
			wantPosition: math2d.Vec2{X: 50, Y: 98},
			wantVelocity: math2d.Vec2{X: 0, Y: -10},
		},
		{
			name:         "Top Edge",
			position:     math2d.Vec2{X: 50, Y: 2},
			velocity:     math2d.Vec2{X: 0, Y: -10},
			wantPosition: math2d.Vec2{X: 50, Y: 2},
			wantVelocity: math2d.Vec2{X: 0, Y: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			world := scene.NewWorld()
			entity := world.Spawn()

			world.SetTransform(entity, scene.Transform{
				Position: tt.position,
				Scale:    math2d.Vec2{X: 1, Y: 1},
			})

			world.SetVelocity(entity, scene.Velocity{
				Linear: tt.velocity,
			})
			world.SetCircleCollider(entity, scene.CircleCollider{
				Radius: 2,
			})

			world.Integrate(0.5)
			world.BounceCirclesInside(bounds)

			transform, ok := world.Transform(entity)
			if !ok {
				t.Fatal("expected transform to exist")
			}

			velocity, ok := world.Velocity(entity)
			if !ok {
				t.Fatal("expected velocity to exist")
			}

			if transform.Position != tt.wantPosition {
				t.Fatalf("got position %+v, want %+v", transform.Position, tt.wantPosition)
			}

			if velocity.Linear != tt.wantVelocity {
				t.Fatalf("got velocity %+v, want %+v", velocity.Linear, tt.wantVelocity)
			}
		})
	}
}

func TestWorldBounceCirclesInsideSkipsEntityWithoutVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 99, Y: 50},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 2,
	})

	world.BounceCirclesInside(math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	))

	transform, ok := world.Transform(entity)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	want := math2d.Vec2{X: 99, Y: 50}
	if transform.Position != want {
		t.Fatalf("got position %+v, want %+v", transform.Position, want)
	}
}

func TestWorldBounceCirclesInsideSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	velocity := scene.Velocity{
		Linear: math2d.Vec2{X: 10, Y: 0},
	}

	world.SetVelocity(entity, velocity)
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 2,
	})

	world.BounceCirclesInside(math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	))

	got, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	if got != velocity {
		t.Fatalf("got velocity %+v, want %+v", got, velocity)
	}
}

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

func TestWorldSeparateCircleOverlapsHandlesSamePosition(t *testing.T) {
	world := scene.NewWorld()

	initialPosition := math2d.Vec2{X: 5, Y: 0}

	first := spawnCollidableCircle(world, initialPosition, 5)
	second := spawnCollidableCircle(world, initialPosition, 5)

	world.SeparateCircleOverlaps()

	firstTransform, ok := world.Transform(first)
	if !ok {
		t.Fatal("expected transform to exist")
	}
	secondTransform, ok := world.Transform(second)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	wantFirstPosition := math2d.Vec2{X: 0, Y: 0}
	wantSecondPosition := math2d.Vec2{X: 10, Y: 0}

	if firstTransform.Position == initialPosition {
		t.Fatal("first transform should have moved")
	}

	if secondTransform.Position == initialPosition {
		t.Fatal("second transform should have moved")
	}

	if firstTransform.Position != wantFirstPosition {
		t.Fatalf("first transform position got %v, want %v", firstTransform.Position, wantFirstPosition)
	}

	if secondTransform.Position != wantSecondPosition {
		t.Fatalf("second transform position got %v, want %v", secondTransform.Position, wantSecondPosition)
	}

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts after separation, want 0", len(contacts))
	}
}

func TestWorldSeparateCircleOverlapsHandlesNearPosition(t *testing.T) {
	world := scene.NewWorld()

	firstInitialPosition := math2d.Vec2{X: 0, Y: 0}
	secondInitialPosition := math2d.Vec2{X: 3, Y: 0}

	first := spawnCollidableCircle(world, firstInitialPosition, 2)
	second := spawnCollidableCircle(world, secondInitialPosition, 2)

	world.SeparateCircleOverlaps()

	firstTransform, ok := world.Transform(first)
	if !ok {
		t.Fatal("expected transform to exist")
	}
	secondTransform, ok := world.Transform(second)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	wantFirstPosition := math2d.Vec2{X: -0.5, Y: 0}
	wantSecondPosition := math2d.Vec2{X: 3.5, Y: 0}

	if firstTransform.Position == firstInitialPosition {
		t.Fatal("first transform should have moved")
	}

	if secondTransform.Position == secondInitialPosition {
		t.Fatal("second transform should have moved")
	}

	if firstTransform.Position != wantFirstPosition {
		t.Fatalf("first transform position got %v, want %v", firstTransform.Position, wantFirstPosition)
	}

	if secondTransform.Position != wantSecondPosition {
		t.Fatalf("second transform position got %v, want %v", secondTransform.Position, wantSecondPosition)
	}

	contacts := world.DetectCircleOverlaps()
	if len(contacts) != 0 {
		t.Fatalf("got %d contacts after separation, want 0", len(contacts))
	}
}

func spawnCollidableCircle(world *scene.World, position math2d.Vec2, radius float64) scene.EntityID {
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: radius,
	})

	return entity
}

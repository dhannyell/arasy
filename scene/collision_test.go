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

func TestWorldDetectCircleOverlapsGridMatchesNarrowPhase(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)
	spawnCollidableCircle(world, math2d.Vec2{X: 25, Y: 0}, 2)
	spawnCollidableCircle(world, math2d.Vec2{X: 26, Y: 0}, 2)

	narrowContacts := world.DetectCircleOverlaps()
	gridContacts := world.DetectCircleOverlapsGrid(16)

	assertContactPairs(t, gridContacts, contactPairs(narrowContacts))
}

func TestWorldDetectCircleOverlapsGridDoesNotDuplicateLargeColliderContacts(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 8, Y: 8}, 10)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 12, Y: 8}, 10)

	contacts := world.DetectCircleOverlapsGrid(4)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
	})
}

func TestWorldDetectCircleOverlapsGridHandlesNegativeCoordinates(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: -3, Y: -3}, 3)
	second := spawnCollidableCircle(world, math2d.Vec2{X: -1, Y: -3}, 3)

	contacts := world.DetectCircleOverlapsGrid(4)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
	})
}

func TestWorldDetectCircleOverlapsGridReturnsContactsInEntityOrder(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)
	third := spawnCollidableCircle(world, math2d.Vec2{X: 6, Y: 0}, 5)

	contacts := world.DetectCircleOverlapsGrid(4)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
		{first, third},
		{second, third},
	})
}

func TestWorldDetectCircleOverlapsGridFallsBackWithInvalidCellSize(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)

	contacts := world.DetectCircleOverlapsGrid(0)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
	})
}

func TestWorldDetectCircleOverlapsGridMarksCandidateDebugCells(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 2, Y: 2}, 0.5)
	spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 2}, 0.5)
	spawnCollidableCircle(world, math2d.Vec2{X: 18, Y: 18}, 0.5)

	world.DetectCircleOverlapsGrid(8)

	var occupied int
	var candidates int
	for _, cell := range world.DebugGridCells() {
		occupied++
		if cell.Candidate {
			candidates++
		}
	}

	if occupied != 2 {
		t.Fatalf("occupied debug cells = %d, want 2", occupied)
	}

	if candidates != 1 {
		t.Fatalf("candidate debug cells = %d, want 1", candidates)
	}
}

func TestWorldResolveCircleOverlapsHandlesSamePosition(t *testing.T) {
	world := scene.NewWorld()

	initialPosition := math2d.Vec2{X: 5, Y: 0}

	first := spawnCollidableCircle(world, initialPosition, 5)
	second := spawnCollidableCircle(world, initialPosition, 5)

	world.ResolveCircleOverlaps()

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

func TestWorldResolveCircleOverlapsHandlesNearPosition(t *testing.T) {
	world := scene.NewWorld()

	firstInitialPosition := math2d.Vec2{X: 0, Y: 0}
	secondInitialPosition := math2d.Vec2{X: 3, Y: 0}

	first := spawnCollidableCircle(world, firstInitialPosition, 2)
	second := spawnCollidableCircle(world, secondInitialPosition, 2)

	world.ResolveCircleOverlaps()

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

func TestWorldResolveCircleOverlapsUpdatesApproachingVelocities(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	world.SetVelocity(first, scene.Velocity{
		Linear: math2d.Vec2{X: 10, Y: 0},
	})
	world.SetVelocity(second, scene.Velocity{
		Linear: math2d.Vec2{X: -10, Y: 0},
	})

	world.ResolveCircleOverlaps()

	firstVelocity, ok := world.Velocity(first)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	secondVelocity, ok := world.Velocity(second)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	wantFirstVelocity := math2d.Vec2{X: -10, Y: 0}
	if firstVelocity.Linear != wantFirstVelocity {
		t.Fatalf("first velocity got %v, want %v", firstVelocity.Linear, wantFirstVelocity)
	}

	wantSecondVelocity := math2d.Vec2{X: 10, Y: 0}
	if secondVelocity.Linear != wantSecondVelocity {
		t.Fatalf("second velocity got %v, want %v", secondVelocity.Linear, wantSecondVelocity)
	}
}

func TestWorldResolveCircleOverlapsKeepsStaticBodyInPlace(t *testing.T) {
	world := scene.NewWorld()

	static := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	dynamic := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	world.SetBody(static, scene.Body{
		InvMass:     0,
		Restitution: 1,
	})
	world.SetBody(dynamic, scene.Body{
		InvMass:     1,
		Restitution: 1,
	})
	world.SetVelocity(static, scene.Velocity{
		Linear: math2d.Vec2{X: 0, Y: 0},
	})
	world.SetVelocity(dynamic, scene.Velocity{
		Linear: math2d.Vec2{X: -10, Y: 0},
	})

	world.ResolveCircleOverlaps()

	staticTransform, ok := world.Transform(static)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	dynamicTransform, ok := world.Transform(dynamic)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	if staticTransform.Position != (math2d.Vec2{X: 0, Y: 0}) {
		t.Fatalf("static position got %v, want {0 0}", staticTransform.Position)
	}

	if dynamicTransform.Position != (math2d.Vec2{X: 4, Y: 0}) {
		t.Fatalf("dynamic position got %v, want {4 0}", dynamicTransform.Position)
	}

	dynamicVelocity, ok := world.Velocity(dynamic)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	if dynamicVelocity.Linear != (math2d.Vec2{X: 10, Y: 0}) {
		t.Fatalf("dynamic velocity got %v, want {10 0}", dynamicVelocity.Linear)
	}
}

func TestWorldResolveCircleOverlapsBouncesOffStaticBodyWithoutVelocity(t *testing.T) {
	world := scene.NewWorld()

	static := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	dynamic := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	world.SetBody(static, scene.Body{
		InvMass:     0,
		Restitution: 1,
	})
	world.SetBody(dynamic, scene.Body{
		InvMass:     1,
		Restitution: 1,
	})
	world.SetVelocity(dynamic, scene.Velocity{
		Linear: math2d.Vec2{X: -10, Y: 0},
	})

	world.ResolveCircleOverlaps()

	staticTransform, ok := world.Transform(static)
	if !ok {
		t.Fatal("expected static transform to exist")
	}
	if staticTransform.Position != (math2d.Vec2{X: 0, Y: 0}) {
		t.Fatalf("static position got %v, want {0 0}", staticTransform.Position)
	}

	dynamicTransform, ok := world.Transform(dynamic)
	if !ok {
		t.Fatal("expected dynamic transform to exist")
	}
	if dynamicTransform.Position != (math2d.Vec2{X: 4, Y: 0}) {
		t.Fatalf("dynamic position got %v, want {4 0}", dynamicTransform.Position)
	}

	dynamicVelocity, ok := world.Velocity(dynamic)
	if !ok {
		t.Fatal("expected dynamic velocity to exist")
	}
	if dynamicVelocity.Linear != (math2d.Vec2{X: 10, Y: 0}) {
		t.Fatalf("dynamic velocity got %v, want {10 0}", dynamicVelocity.Linear)
	}

	if _, ok := world.Velocity(static); ok {
		t.Fatal("static velocity exists, want absent")
	}
}

func TestWorldResolveCircleOverlapsUsesRestitution(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	world.SetBody(first, scene.Body{
		InvMass:     1,
		Restitution: 0,
	})
	world.SetBody(second, scene.Body{
		InvMass:     1,
		Restitution: 0,
	})
	world.SetVelocity(first, scene.Velocity{
		Linear: math2d.Vec2{X: 10, Y: 0},
	})
	world.SetVelocity(second, scene.Velocity{
		Linear: math2d.Vec2{X: -10, Y: 0},
	})

	world.ResolveCircleOverlaps()

	firstVelocity, ok := world.Velocity(first)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	secondVelocity, ok := world.Velocity(second)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	if firstVelocity.Linear != (math2d.Vec2{X: 0, Y: 0}) {
		t.Fatalf("first velocity got %v, want {0 0}", firstVelocity.Linear)
	}

	if secondVelocity.Linear != (math2d.Vec2{X: 0, Y: 0}) {
		t.Fatalf("second velocity got %v, want {0 0}", secondVelocity.Linear)
	}
}

func TestWorldResolveCircleOverlapsSkipsTwoStaticBodies(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 2)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 2)

	world.SetBody(first, scene.Body{
		InvMass:     0,
		Restitution: 1,
	})
	world.SetBody(second, scene.Body{
		InvMass:     0,
		Restitution: 1,
	})

	world.ResolveCircleOverlaps()

	firstTransform, ok := world.Transform(first)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	secondTransform, ok := world.Transform(second)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	if firstTransform.Position != (math2d.Vec2{X: 0, Y: 0}) {
		t.Fatalf("first position got %v, want {0 0}", firstTransform.Position)
	}

	if secondTransform.Position != (math2d.Vec2{X: 3, Y: 0}) {
		t.Fatalf("second position got %v, want {3 0}", secondTransform.Position)
	}
}

func assertContactPairs(t *testing.T, contacts []scene.CircleContact, want [][2]scene.EntityID) {
	t.Helper()

	if len(contacts) != len(want) {
		t.Fatalf("got %d contacts, want %d", len(contacts), len(want))
	}

	for i := range want {
		if contacts[i].A != want[i][0] || contacts[i].B != want[i][1] {
			t.Fatalf(
				"contact %d got pair (%d, %d), want (%d, %d)",
				i,
				contacts[i].A,
				contacts[i].B,
				want[i][0],
				want[i][1],
			)
		}
	}
}

func contactPairs(contacts []scene.CircleContact) [][2]scene.EntityID {
	pairs := make([][2]scene.EntityID, 0, len(contacts))
	for _, contact := range contacts {
		pairs = append(pairs, [2]scene.EntityID{contact.A, contact.B})
	}

	return pairs
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
	world.SetBody(entity, scene.Body{
		InvMass:     1,
		Restitution: 1,
	})

	return entity
}

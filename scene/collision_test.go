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
			world.BounceEntitiesInside(bounds, scene.BounceConfig{})

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

func TestWorldBounceRectsInsideBounces(t *testing.T) {
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
			position:     math2d.Vec2{X: 97, Y: 50},
			velocity:     math2d.Vec2{X: 10, Y: 0},
			wantPosition: math2d.Vec2{X: 97, Y: 50},
			wantVelocity: math2d.Vec2{X: -10, Y: 0},
		},
		{
			name:         "Left Edge",
			position:     math2d.Vec2{X: 3, Y: 50},
			velocity:     math2d.Vec2{X: -10, Y: 0},
			wantPosition: math2d.Vec2{X: 3, Y: 50},
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
			world.SetRectCollider(entity, scene.RectCollider{
				HalfExtends: math2d.Vec2{X: 3, Y: 2},
			})

			world.Integrate(0.5)
			world.BounceEntitiesInside(bounds, scene.BounceConfig{})

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

func TestWorldBounceEntitiesInsidePrefersCircleWhenBothColliders(t *testing.T) {
	bounds := math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	)

	world := scene.NewWorld()
	entity := world.Spawn()

	position := math2d.Vec2{X: 97, Y: 50}
	velocity := scene.Velocity{Linear: math2d.Vec2{X: 5, Y: 0}}

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetVelocity(entity, velocity)
	world.SetCircleCollider(entity, scene.CircleCollider{Radius: 2})
	world.SetRectCollider(entity, scene.RectCollider{HalfExtends: math2d.Vec2{X: 10, Y: 10}})

	world.BounceEntitiesInside(bounds, scene.BounceConfig{})

	transform, ok := world.Transform(entity)
	if !ok {
		t.Fatal("expected transform to exist")
	}
	if transform.Position != position {
		t.Fatalf("got position %+v, want %+v (circle collider should win, no clamp expected)", transform.Position, position)
	}

	gotVelocity, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to exist")
	}
	if gotVelocity != velocity {
		t.Fatalf("got velocity %+v, want %+v (circle collider should win, no bounce expected)", gotVelocity, velocity)
	}
}

func TestWorldBounceEntitiesInsideOpenSides(t *testing.T) {
	bounds := math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	)

	tests := []struct {
		name         string
		openSide     scene.SideMask
		position     math2d.Vec2
		velocity     math2d.Vec2
		wantClosed   math2d.Vec2
		wantVelocity math2d.Vec2
	}{
		{
			name:         "Left",
			openSide:     scene.SideLeft,
			position:     math2d.Vec2{X: 1, Y: 50},
			velocity:     math2d.Vec2{X: -5, Y: 0},
			wantClosed:   math2d.Vec2{X: 2, Y: 50},
			wantVelocity: math2d.Vec2{X: 5, Y: 0},
		},
		{
			name:         "Right",
			openSide:     scene.SideRight,
			position:     math2d.Vec2{X: 99, Y: 50},
			velocity:     math2d.Vec2{X: 5, Y: 0},
			wantClosed:   math2d.Vec2{X: 98, Y: 50},
			wantVelocity: math2d.Vec2{X: -5, Y: 0},
		},
		{
			name:         "Top",
			openSide:     scene.SideTop,
			position:     math2d.Vec2{X: 50, Y: 1},
			velocity:     math2d.Vec2{X: 0, Y: -5},
			wantClosed:   math2d.Vec2{X: 50, Y: 2},
			wantVelocity: math2d.Vec2{X: 0, Y: 5},
		},
		{
			name:         "Bottom",
			openSide:     scene.SideBottom,
			position:     math2d.Vec2{X: 50, Y: 99},
			velocity:     math2d.Vec2{X: 0, Y: 5},
			wantClosed:   math2d.Vec2{X: 50, Y: 98},
			wantVelocity: math2d.Vec2{X: 0, Y: -5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/Open", func(t *testing.T) {
			world, entity := worldWithCircleAt(tt.position, tt.velocity)

			world.BounceEntitiesInside(bounds, scene.BounceConfig{OpenSides: tt.openSide})

			transform, _ := world.Transform(entity)
			velocity, _ := world.Velocity(entity)
			if transform.Position != tt.position {
				t.Fatalf("open side position = %+v, want %+v", transform.Position, tt.position)
			}
			if velocity.Linear != tt.velocity {
				t.Fatalf("open side velocity = %+v, want %+v", velocity.Linear, tt.velocity)
			}
		})

		t.Run(tt.name+"/Closed", func(t *testing.T) {
			world, entity := worldWithCircleAt(tt.position, tt.velocity)

			world.BounceEntitiesInside(bounds, scene.BounceConfig{})

			transform, _ := world.Transform(entity)
			velocity, _ := world.Velocity(entity)
			if transform.Position != tt.wantClosed {
				t.Fatalf("closed side position = %+v, want %+v", transform.Position, tt.wantClosed)
			}
			if velocity.Linear != tt.wantVelocity {
				t.Fatalf("closed side velocity = %+v, want %+v", velocity.Linear, tt.wantVelocity)
			}
		})
	}
}

func TestWorldBounceEntitiesInsideWallRestitution(t *testing.T) {
	bounds := math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	)

	t.Run("UsesBodyRestitutionWhenEnabled", func(t *testing.T) {
		world, entity := worldWithCircleAt(math2d.Vec2{X: 99, Y: 50}, math2d.Vec2{X: 10, Y: 0})
		world.SetBody(entity, scene.Body{InvMass: 1, Restitution: 0.5})

		world.BounceEntitiesInside(bounds, scene.BounceConfig{UseBodyRestitution: true})

		velocity, _ := world.Velocity(entity)
		if velocity.Linear != (math2d.Vec2{X: -5, Y: 0}) {
			t.Fatalf("velocity = %+v, want {-5 0}", velocity.Linear)
		}
	})

	t.Run("EntityWithoutBodyStillReflectsPerfectly", func(t *testing.T) {
		world, entity := worldWithCircleAt(math2d.Vec2{X: 99, Y: 50}, math2d.Vec2{X: 10, Y: 0})

		world.BounceEntitiesInside(bounds, scene.BounceConfig{UseBodyRestitution: true})

		velocity, _ := world.Velocity(entity)
		if velocity.Linear != (math2d.Vec2{X: -10, Y: 0}) {
			t.Fatalf("velocity = %+v, want {-10 0}", velocity.Linear)
		}
	})

	t.Run("DefaultIgnoresBodyRestitution", func(t *testing.T) {
		world, entity := worldWithCircleAt(math2d.Vec2{X: 99, Y: 50}, math2d.Vec2{X: 10, Y: 0})
		world.SetBody(entity, scene.Body{InvMass: 1, Restitution: 0})

		world.BounceEntitiesInside(bounds, scene.BounceConfig{})

		velocity, _ := world.Velocity(entity)
		if velocity.Linear != (math2d.Vec2{X: -10, Y: 0}) {
			t.Fatalf("velocity = %+v, want {-10 0}", velocity.Linear)
		}
	})
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

	world.BounceEntitiesInside(math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	), scene.BounceConfig{})

	transform, ok := world.Transform(entity)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	want := math2d.Vec2{X: 99, Y: 50}
	if transform.Position != want {
		t.Fatalf("got position %+v, want %+v", transform.Position, want)
	}
}

func worldWithCircleAt(position, velocity math2d.Vec2) (*scene.World, scene.EntityID) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetVelocity(entity, scene.Velocity{Linear: velocity})
	world.SetCircleCollider(entity, scene.CircleCollider{Radius: 2})

	return world, entity
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

	world.BounceEntitiesInside(math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	), scene.BounceConfig{})

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

// TestWorldDetectOverlapsGridMatchesAllPairsWithRects proves the grid
// broad-phase agrees with an all-pairs oracle on a world mixing circles and
// rects. DetectCircleOverlaps() cannot be used as the oracle here: it only
// builds pairs from circleColliderEntities() and ignores rects entirely, so
// it would silently drop every rect-involving contact. Instead, the oracle is
// the grid itself with a single giant cell (cellSize 10000 for a 60x60
// world), which forces it to fall back to comparing every pair — equivalent
// to all-pairs, but through the code path that actually sees rects.
func TestWorldDetectOverlapsGridMatchesAllPairsWithRects(t *testing.T) {
	world := benchmarkWorldSpreadMixed(36, 60, 60, 6, 2)
	clone := world.Clone()

	small := world.DetectCircleOverlapsGrid(8)
	large := clone.DetectCircleOverlapsGrid(10000)

	if len(small) == 0 {
		t.Fatal("expected at least one contact, got 0")
	}

	if len(small) != len(large) {
		t.Fatalf("got %d contacts from small grid, %d from single-cell grid", len(small), len(large))
	}

	for i := range small {
		if small[i].A != large[i].A || small[i].B != large[i].B {
			t.Fatalf("contact %d got pair (%d,%d), want (%d,%d)", i, small[i].A, small[i].B, large[i].A, large[i].B)
		}
		if small[i].Normal != large[i].Normal {
			t.Fatalf("contact %d got normal %+v, want %+v", i, small[i].Normal, large[i].Normal)
		}
		if small[i].Depth != large[i].Depth {
			t.Fatalf("contact %d got depth %v, want %v", i, small[i].Depth, large[i].Depth)
		}
	}
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

func TestWorldResolveCircleVsRectUsesMinRestitution(t *testing.T) {
	tests := []struct {
		name         string
		restitution  float64
		wantVelocity math2d.Vec2
	}{
		{
			// min(circle=1, rect=0) = 0: normal velocity is absorbed, not reflected.
			name:         "AbsorbsWithZeroRestitution",
			restitution:  0,
			wantVelocity: math2d.Vec2{X: 0, Y: 0},
		},
		{
			// min(circle=1, rect=1) = 1: fully reflected. The rect is static
			// (InvMass 0), so totalInvMass == circle's InvMass and the impulse
			// lands entirely on the circle.
			name:         "ReflectsWithFullRestitution",
			restitution:  1,
			wantVelocity: math2d.Vec2{X: -10, Y: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			world := scene.NewWorld()

			circle := spawnCollidableCircle(world, math2d.Vec2{X: 7, Y: 0}, 2)
			rect := spawnCollidableRect(world, math2d.Vec2{X: 10, Y: 0}, math2d.Vec2{X: 2, Y: 2})

			world.SetBody(circle, scene.Body{InvMass: 1, Restitution: 1})
			world.SetBody(rect, scene.Body{InvMass: 0, Restitution: tt.restitution})
			world.SetVelocity(circle, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 0}})

			world.ResolveCircleOverlapsGrid(8)

			velocity, ok := world.Velocity(circle)
			if !ok {
				t.Fatal("expected velocity to exist")
			}
			if velocity.Linear != tt.wantVelocity {
				t.Fatalf("got velocity %+v, want %+v", velocity.Linear, tt.wantVelocity)
			}

			transform, ok := world.Transform(circle)
			if !ok {
				t.Fatal("expected transform to exist")
			}
			wantPosition := math2d.Vec2{X: 6, Y: 0} // pushed out by depth 1 along -X (A is circle, normal points A->B = +X)
			if transform.Position != wantPosition {
				t.Fatalf("got position %+v, want %+v", transform.Position, wantPosition)
			}
		})
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
	// have its normal velocity reflected — no teleport.
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

func assertContactPairs(t *testing.T, contacts []scene.Contact, want [][2]scene.EntityID) {
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

func contactPairs(contacts []scene.Contact) [][2]scene.EntityID {
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

func spawnCollidableRect(world *scene.World, position, halfExtends math2d.Vec2) scene.EntityID {
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetRectCollider(entity, scene.RectCollider{
		HalfExtends: halfExtends,
	})
	world.SetBody(entity, scene.Body{
		InvMass:     1,
		Restitution: 1,
	})

	return entity
}

func spawnStaticRect(world *scene.World, position, halfExtends math2d.Vec2) scene.EntityID {
	entity := spawnCollidableRect(world, position, halfExtends)
	world.SetBody(entity, scene.Body{
		InvMass:     0,
		Restitution: 1,
	})

	return entity
}

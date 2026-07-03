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

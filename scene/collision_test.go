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

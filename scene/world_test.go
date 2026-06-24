package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

func TestNewWorldInitializesComponentStores(t *testing.T) {
	world := scene.NewWorld()

	if world == nil {
		t.Fatal("NewWorld returned nil")
	}

	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{})
	world.SetCircleSprite(entity, scene.CircleSprite{})
	world.SetVelocity(entity, scene.Velocity{})
	world.SetCircleCollider(entity, scene.CircleCollider{})
}

func TestWorldSpawnReturnsUniqueEntityIDs(t *testing.T) {
	world := scene.NewWorld()

	first := world.Spawn()
	second := world.Spawn()

	if first == 0 {
		t.Fatal("first entity ID should not be zero")
	}

	if first == second {
		t.Fatalf("entity IDs should be unique, got %d twice", first)
	}
}

func TestWorldStoresAndReadsTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	transform := scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 20},
		Rotation: 1.5,
		Scale:    math2d.Vec2{X: 2, Y: 3},
	}

	world.SetTransform(entity, transform)

	got, ok := world.Transform(entity)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	if got != transform {
		t.Fatalf("got %+v, want %+v", got, transform)
	}
}

func TestWorldStoresAndReadsCircleSprite(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	sprite := scene.CircleSprite{
		Radius: 1.25,
		Color:  render.RGBA(0.2, 0.4, 0.6, 1),
		Layer:  8,
	}

	world.SetCircleSprite(entity, sprite)

	got, ok := world.CircleSprite(entity)
	if !ok {
		t.Fatal("expected circle sprite to exist")
	}

	if got != sprite {
		t.Fatalf("got %+v, want %+v", got, sprite)
	}
}

func TestWorldStoresAndReadsVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	velocity := scene.Velocity{
		Linear: math2d.Vec2{X: 3, Y: -2},
	}

	world.SetVelocity(entity, velocity)

	got, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	if got != velocity {
		t.Fatalf("got %+v, want %+v", got, velocity)
	}
}

func TestWorldRemovesComponents(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{})
	world.SetCircleSprite(entity, scene.CircleSprite{})
	world.SetVelocity(entity, scene.Velocity{})
	world.SetCircleCollider(entity, scene.CircleCollider{})

	world.RemoveTransform(entity)
	world.RemoveCircleSprite(entity)
	world.RemoveVelocity(entity)
	world.RemoveCircleCollider(entity)

	if _, ok := world.Transform(entity); ok {
		t.Fatal("expected transform to be removed")
	}

	if _, ok := world.CircleSprite(entity); ok {
		t.Fatal("expected circle sprite to be removed")
	}

	if _, ok := world.Velocity(entity); ok {
		t.Fatal("expected velocity to be removed")
	}

	if _, ok := world.CircleCollider(entity); ok {
		t.Fatal("expected circle collider to be removed")
	}
}

func TestWorldDespawnRemovesAllEntityComponents(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{})
	world.SetCircleSprite(entity, scene.CircleSprite{})
	world.SetVelocity(entity, scene.Velocity{})
	world.SetCircleCollider(entity, scene.CircleCollider{})

	world.Despawn(entity)

	if _, ok := world.Transform(entity); ok {
		t.Fatal("expected transform to be removed")
	}

	if _, ok := world.CircleSprite(entity); ok {
		t.Fatal("expected circle sprite to be removed")
	}

	if _, ok := world.Velocity(entity); ok {
		t.Fatal("expected velocity to be removed")
	}

	if _, ok := world.CircleCollider(entity); ok {
		t.Fatal("expected circle collider to be removed")
	}
}

func TestWorldIntegrateMovesEntityWithTransformAndVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 20},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetVelocity(entity, scene.Velocity{
		Linear: math2d.Vec2{X: 4, Y: -2},
	})

	world.Integrate(0.5)

	transform, ok := world.Transform(entity)
	if !ok {
		t.Fatal("expected transform to exist")
	}

	want := math2d.Vec2{X: 12, Y: 19}
	if transform.Position != want {
		t.Fatalf("got position %+v, want %+v", transform.Position, want)
	}
}

func TestWorldIntegrateSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetVelocity(entity, scene.Velocity{
		Linear: math2d.Vec2{X: 4, Y: -2},
	})

	world.Integrate(0.5)

	if _, ok := world.Transform(entity); ok {
		t.Fatal("expected transform to stay absent")
	}

	velocity, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to stay present")
	}

	want := scene.Velocity{Linear: math2d.Vec2{X: 4, Y: -2}}
	if velocity != want {
		t.Fatalf("got velocity %+v, want %+v", velocity, want)
	}
}

func TestWorldRenderCommandsCreatesCircleForRenderableEntity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	position := math2d.Vec2{X: 4, Y: 7}
	color := render.RGBA(1, 0, 0, 1)

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: 2,
		Color:  color,
		Layer:  5,
	})

	commands := world.RenderCommands()
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}

	command, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}

	if command.Center != position {
		t.Fatalf("got center %+v, want %+v", command.Center, position)
	}

	if command.Radius != 2 {
		t.Fatalf("got radius %v, want 2", command.Radius)
	}

	if command.Fill != color {
		t.Fatalf("got color %+v, want %+v", command.Fill, color)
	}

	if command.Layer != 5 {
		t.Fatalf("got layer %d, want 5", command.Layer)
	}
}

func TestWorldRenderCommandsSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: 2,
		Color:  render.RGBA(1, 0, 0, 1),
		Layer:  5,
	})

	commands := world.RenderCommands()
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

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

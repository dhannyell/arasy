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

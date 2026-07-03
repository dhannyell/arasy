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
	world.SetAcceleration(entity, scene.Acceleration{})
	world.SetDamping(entity, scene.Damping{})
	world.SetBody(entity, scene.Body{})
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

func TestWorldCloneCopiesComponentsWithoutSharingMaps(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	transform := scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 20},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	}
	velocity := scene.Velocity{
		Linear: math2d.Vec2{X: 3, Y: 4},
	}

	world.SetTransform(entity, transform)
	world.SetVelocity(entity, velocity)
	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: 2,
		BaseSprite: scene.BaseSprite{
			Color: render.RGBA(1, 0, 0, 1),
			Layer: 5,
		},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 1.5,
	})
	world.DetectCircleOverlapsGrid(4)
	world.SetBody(entity, scene.Body{
		InvMass:     0.5,
		Restitution: 0.25,
	})

	clone := world.Clone()

	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 99, Y: 99},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.RemoveVelocity(entity)
	world.DetectCircleOverlapsGrid(0)

	gotTransform, ok := clone.Transform(entity)
	if !ok {
		t.Fatal("expected clone to keep transform")
	}

	if gotTransform != transform {
		t.Fatalf("got transform %+v, want %+v", gotTransform, transform)
	}

	gotVelocity, ok := clone.Velocity(entity)
	if !ok {
		t.Fatal("expected clone to keep velocity")
	}

	if gotVelocity != velocity {
		t.Fatalf("got velocity %+v, want %+v", gotVelocity, velocity)
	}

	next := clone.Spawn()
	if next == entity {
		t.Fatal("expected cloned world to preserve next entity id")
	}

	if len(clone.DebugGridCells()) == 0 {
		t.Fatal("expected clone to keep debug grid cells")
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

func TestWorldStepTickCountsSimulationSteps(t *testing.T) {
	world := scene.NewWorld()

	if world.StepTick() != 0 {
		t.Fatalf("initial StepTick = %d, want 0", world.StepTick())
	}

	world.Step(1.0/60.0, scene.StepConfig{})

	if world.StepTick() != 1 {
		t.Fatalf("StepTick after step = %d, want 1", world.StepTick())
	}
}

func TestWorldStoresAndReadsCircleSprite(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	sprite := scene.CircleSprite{
		Radius: 1.25,
		BaseSprite: scene.BaseSprite{
			Color: render.RGBA(0.2, 0.4, 0.6, 1),
			Layer: 8,
		},
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

func TestWorldStoresAndReadsAcceleration(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	acceleration := scene.Acceleration{
		Linear: math2d.Vec2{X: 3, Y: -2},
	}

	world.SetAcceleration(entity, acceleration)

	got, ok := world.Acceleration(entity)
	if !ok {
		t.Fatal("expected acceleration to exist")
	}

	if got != acceleration {
		t.Fatalf("got %+v, want %+v", got, acceleration)
	}
}

func TestWorldStoresAndReadsDamping(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	damping := scene.Damping{
		Linear: 0.5,
	}

	world.SetDamping(entity, damping)

	got, ok := world.Damping(entity)
	if !ok {
		t.Fatal("expected damping to exist")
	}

	if got != damping {
		t.Fatalf("got %+v, want %+v", got, damping)
	}
}

func TestWorldStoresAndReadsBody(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	body := scene.Body{
		InvMass:     0.5,
		Restitution: 0.25,
	}

	world.SetBody(entity, body)

	got, ok := world.Body(entity)
	if !ok {
		t.Fatal("expected body to exist")
	}

	if got != body {
		t.Fatalf("got %+v, want %+v", got, body)
	}
}

func TestWorldRemovesComponents(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{})
	world.SetCircleSprite(entity, scene.CircleSprite{})
	world.SetVelocity(entity, scene.Velocity{})
	world.SetAcceleration(entity, scene.Acceleration{})
	world.SetDamping(entity, scene.Damping{})
	world.SetBody(entity, scene.Body{})
	world.SetCircleCollider(entity, scene.CircleCollider{})

	world.RemoveTransform(entity)
	world.RemoveCircleSprite(entity)
	world.RemoveVelocity(entity)
	world.RemoveAcceleration(entity)
	world.RemoveDamping(entity)
	world.RemoveBody(entity)
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

	if _, ok := world.Acceleration(entity); ok {
		t.Fatal("expected acceleration to be removed")
	}

	if _, ok := world.Damping(entity); ok {
		t.Fatal("expected damping to be removed")
	}

	if _, ok := world.Body(entity); ok {
		t.Fatal("expected body to be removed")
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
	world.SetAcceleration(entity, scene.Acceleration{})
	world.SetDamping(entity, scene.Damping{})
	world.SetBody(entity, scene.Body{})
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

	if _, ok := world.Acceleration(entity); ok {
		t.Fatal("expected acceleration to be removed")
	}

	if _, ok := world.Damping(entity); ok {
		t.Fatal("expected damping to be removed")
	}

	if _, ok := world.Body(entity); ok {
		t.Fatal("expected body to be removed")
	}

	if _, ok := world.CircleCollider(entity); ok {
		t.Fatal("expected circle collider to be removed")
	}

	world.DetectCircleOverlapsGrid(4)
}

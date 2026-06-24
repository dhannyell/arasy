package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

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

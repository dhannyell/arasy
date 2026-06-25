package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func TestWorldStepIsDeterministic(t *testing.T) {
	first := newStepTestWorld()
	second := newStepTestWorld()

	config := scene.StepConfig{
		Bounds: math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: 100, Y: 100},
		),
	}

	for range 120 {
		first.Step(1.0/60.0, config)
		second.Step(1.0/60.0, config)
	}

	assertWorldState(t, first, second)
}

func newStepTestWorld() *scene.World {
	world := scene.NewWorld()

	spawnStepTestCircle(
		world,
		math2d.Vec2{X: 20, Y: 50},
		math2d.Vec2{X: 18, Y: 0},
		3,
	)
	spawnStepTestCircle(
		world,
		math2d.Vec2{X: 80, Y: 50},
		math2d.Vec2{X: -18, Y: 0},
		3,
	)
	spawnStepTestCircle(
		world,
		math2d.Vec2{X: 50, Y: 15},
		math2d.Vec2{X: 0, Y: 14},
		2,
	)

	return world
}

func spawnStepTestCircle(world *scene.World, position, velocity math2d.Vec2, radius float64) scene.EntityID {
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetVelocity(entity, scene.Velocity{
		Linear: velocity,
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: radius,
	})

	return entity
}

func assertWorldState(t *testing.T, got, want *scene.World) {
	t.Helper()

	if len(got.Transforms) != len(want.Transforms) {
		t.Fatalf("got %d transforms, want %d", len(got.Transforms), len(want.Transforms))
	}

	for id, wantTransform := range want.Transforms {
		gotTransform, ok := got.Transforms[id]
		if !ok {
			t.Fatalf("missing transform for entity %d", id)
		}

		if gotTransform != wantTransform {
			t.Fatalf("entity %d transform got %+v, want %+v", id, gotTransform, wantTransform)
		}
	}

	if len(got.Velocities) != len(want.Velocities) {
		t.Fatalf("got %d velocities, want %d", len(got.Velocities), len(want.Velocities))
	}

	for id, wantVelocity := range want.Velocities {
		gotVelocity, ok := got.Velocities[id]
		if !ok {
			t.Fatalf("missing velocity for entity %d", id)
		}

		if gotVelocity != wantVelocity {
			t.Fatalf("entity %d velocity got %+v, want %+v", id, gotVelocity, wantVelocity)
		}
	}
}

package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
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

func TestWorldApplyAccelerationChangesVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetVelocity(entity, scene.Velocity{
		Linear: math2d.Vec2{X: 10, Y: -5},
	})
	world.SetAcceleration(entity, scene.Acceleration{
		Linear: math2d.Vec2{X: 2, Y: 4},
	})

	world.ApplyAcceleration(0.5)

	velocity, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	want := math2d.Vec2{X: 11, Y: -3}
	if velocity.Linear != want {
		t.Fatalf("got velocity %+v, want %+v", velocity.Linear, want)
	}
}

func TestWorldApplyAccelerationSkipsEntityWithoutVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetAcceleration(entity, scene.Acceleration{
		Linear: math2d.Vec2{X: 2, Y: 4},
	})

	world.ApplyAcceleration(0.5)

	if _, ok := world.Velocity(entity); ok {
		t.Fatal("expected velocity to stay absent")
	}
}

func TestWorldApplyDampingReducesVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetVelocity(entity, scene.Velocity{
		Linear: math2d.Vec2{X: 10, Y: -5},
	})
	world.SetDamping(entity, scene.Damping{
		Linear: 0.5,
	})

	world.ApplyDamping(1)

	velocity, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	want := math2d.Vec2{X: 5, Y: -2.5}
	if velocity.Linear != want {
		t.Fatalf("got velocity %+v, want %+v", velocity.Linear, want)
	}
}

func TestWorldApplyDampingClampsAtZero(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetVelocity(entity, scene.Velocity{
		Linear: math2d.Vec2{X: 10, Y: -5},
	})
	world.SetDamping(entity, scene.Damping{
		Linear: 2,
	})

	world.ApplyDamping(1)

	velocity, ok := world.Velocity(entity)
	if !ok {
		t.Fatal("expected velocity to exist")
	}

	want := math2d.Vec2{X: 0, Y: 0}
	if velocity.Linear != want {
		t.Fatalf("got velocity %+v, want %+v", velocity.Linear, want)
	}
}

func TestWorldApplyDampingNeverAccelerates(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()
	world.SetVelocity(entity, scene.Velocity{
		Linear: math2d.Vec2{X: 3, Y: -4},
	})
	world.SetDamping(entity, scene.Damping{Linear: -1e6})

	world.ApplyDamping(1)

	velocity, _ := world.Velocity(entity)
	if velocity.Linear != (math2d.Vec2{X: 3, Y: -4}) {
		t.Fatalf("negative damping accelerated velocity: %+v", velocity.Linear)
	}
}

func TestWorldApplyDampingSkipsEntityWithoutVelocity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetDamping(entity, scene.Damping{
		Linear: 0.5,
	})

	world.ApplyDamping(1)

	if _, ok := world.Velocity(entity); ok {
		t.Fatal("expected velocity to stay absent")
	}
}

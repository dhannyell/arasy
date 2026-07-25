package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

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

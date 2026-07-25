package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

func TestWorldStepIsDeterministic(t *testing.T) {
	first, firstEntities := newStepTestWorld()
	second, secondEntities := newStepTestWorld()

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

	assertWorldState(t, first, firstEntities, second, secondEntities)
}

func TestWorldStepUsesConfiguredGridCellSize(t *testing.T) {
	world := scene.NewWorld()

	spawnStepTestCircle(
		world,
		math2d.Vec2{X: 8, Y: 8},
		math2d.Vec2{},
		1,
	)

	config := scene.StepConfig{
		Bounds: math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: 100, Y: 100},
		),
		Physics: scene.PhysicsConfig{
			CellSize: 4,
		},
	}

	world.Step(1.0/60.0, config)

	if len(world.DebugGridCells()) == 0 {
		t.Fatal("expected step to populate debug grid cells")
	}

	for _, cell := range world.DebugGridCells() {
		if cell.Size != 4 {
			t.Fatalf("debug cell size = %v, want 4", cell.Size)
		}

		if cell.X < 1 || cell.X > 2 || cell.Y < 1 || cell.Y > 2 {
			t.Fatalf("debug cell %+v does not match configured cell size", cell)
		}
	}
}

func newStepTestWorld() (*scene.World, []scene.EntityID) {
	world := scene.NewWorld()

	first := spawnStepTestCircle(
		world,
		math2d.Vec2{X: 20, Y: 50},
		math2d.Vec2{X: 18, Y: 0},
		3,
	)
	second := spawnStepTestCircle(
		world,
		math2d.Vec2{X: 80, Y: 50},
		math2d.Vec2{X: -18, Y: 0},
		3,
	)
	third := spawnStepTestCircle(
		world,
		math2d.Vec2{X: 50, Y: 15},
		math2d.Vec2{X: 0, Y: 14},
		2,
	)

	return world, []scene.EntityID{first, second, third}
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

func assertWorldState(t *testing.T, got *scene.World, gotEntities []scene.EntityID, want *scene.World, wantEntities []scene.EntityID) {
	t.Helper()

	if len(gotEntities) != len(wantEntities) {
		t.Fatalf("got %d entities, want %d", len(gotEntities), len(wantEntities))
	}

	for i, gotID := range gotEntities {
		wantID := wantEntities[i]

		wantTransform, ok := want.Transform(wantID)
		if !ok {
			t.Fatalf("missing want transform for entity %d", wantID)
		}

		gotTransform, ok := got.Transform(gotID)
		if !ok {
			t.Fatalf("missing got transform for entity %d", gotID)
		}

		if gotTransform != wantTransform {
			t.Fatalf("entity %d transform got %+v, want %+v", gotID, gotTransform, wantTransform)
		}

		wantVelocity, wantOK := want.Velocity(wantID)
		gotVelocity, gotOK := got.Velocity(gotID)

		if gotOK != wantOK {
			t.Fatalf("entity %d velocity presence got %v, want %v", gotID, gotOK, wantOK)
		}

		if !wantOK {
			continue
		}

		if gotVelocity != wantVelocity {
			t.Fatalf("entity %d velocity got %+v, want %+v", gotID, gotVelocity, wantVelocity)
		}
	}
}

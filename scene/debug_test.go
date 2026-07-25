package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestWorldDebugVelocityCommandsCreatesLineForMovingEntity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	position := math2d.Vec2{X: 4, Y: 7}
	velocity := math2d.Vec2{X: 2, Y: 0}
	color := render.RGBA(0.5, 0.5, 0.5, 1)

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetVelocity(entity, scene.Velocity{Linear: velocity})

	commands := world.DebugVelocityCommands(color, 0.3, 2, 103)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}

	command, ok := commands[0].(render.DrawLine)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawLine", commands[0])
	}

	if command.From != position {
		t.Fatalf("got from %+v, want %+v", command.From, position)
	}

	want := position.Add(velocity.Mul(2))
	if command.To != want {
		t.Fatalf("got to %+v, want %+v", command.To, want)
	}

	if command.Color != color {
		t.Fatalf("got color %+v, want %+v", command.Color, color)
	}

	if command.Width != 0.3 {
		t.Fatalf("got width %v, want 0.3", command.Width)
	}

	if command.Layer() != 103 {
		t.Fatalf("got layer %d, want 103", command.Layer())
	}
}

func TestWorldDebugVelocityCommandsSkipsStationaryEntity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetVelocity(entity, scene.Velocity{})

	commands := world.DebugVelocityCommands(render.RGBA(1, 1, 1, 1), 0.3, 2, 103)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func TestWorldDebugVelocityCommandsSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 1}})

	commands := world.DebugVelocityCommands(render.RGBA(1, 1, 1, 1), 0.3, 2, 103)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func TestWorldDebugVelocityCommandsReturnsNoCommandsWithInvalidInput(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 1}})

	if commands := world.DebugVelocityCommands(render.RGBA(1, 1, 1, 1), 0, 2, 103); len(commands) != 0 {
		t.Fatalf("got %d commands with width<=0, want 0", len(commands))
	}

	if commands := world.DebugVelocityCommands(render.RGBA(1, 1, 1, 1), 0.3, 0, 103); len(commands) != 0 {
		t.Fatalf("got %d commands with scale==0, want 0", len(commands))
	}
}

func TestWorldDebugVelocityCommandsOrdersByEntityID(t *testing.T) {
	world := scene.NewWorld()

	var ids []scene.EntityID
	for i := 0; i < 5; i++ {
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
		world.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})
		ids = append(ids, id)
	}

	commands := world.DebugVelocityCommands(render.RGBA(1, 1, 1, 1), 0.3, 1, 0)
	if len(commands) != len(ids) {
		t.Fatalf("got %d commands, want %d", len(commands), len(ids))
	}

	for i := 1; i < len(ids); i++ {
		if !(ids[i-1] < ids[i]) {
			t.Fatalf("entity ids not sorted ascending: %v", ids)
		}
	}
}

func TestWorldDebugEntityIDCommandsCreatesLabelPerEntity(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	position := math2d.Vec2{X: 4, Y: 7}
	offset := math2d.Vec2{X: 0.5, Y: -0.5}
	color := render.RGBA(1, 1, 1, 0.5)

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})

	commands := world.DebugEntityIDCommands(color, 2, offset, 104)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}

	command, ok := commands[0].(render.DrawText)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawText", commands[0])
	}

	wantPos := position.Add(offset)
	if command.Position != wantPos {
		t.Fatalf("got position %+v, want %+v", command.Position, wantPos)
	}

	if command.Text != "1" {
		t.Fatalf("got text %q, want %q", command.Text, "1")
	}

	if command.Color != color {
		t.Fatalf("got color %+v, want %+v", command.Color, color)
	}

	if command.Size != 2 {
		t.Fatalf("got size %v, want 2", command.Size)
	}

	if command.Layer() != 104 {
		t.Fatalf("got layer %d, want 104", command.Layer())
	}
}

func TestWorldDebugEntityIDCommandsReturnsNoCommandsWithInvalidSize(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})

	commands := world.DebugEntityIDCommands(render.RGBA(1, 1, 1, 1), 0, math2d.Vec2{}, 104)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func TestWorldDebugCommandsRespectsLayerFlags(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 1, Y: 1},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{Radius: 1})
	world.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})

	none := world.DebugCommands(0)
	if len(none) != 0 {
		t.Fatalf("got %d commands with no layers enabled, want 0", len(none))
	}

	colliders := world.DebugCommands(scene.DebugColliders)
	if len(colliders) != 1 {
		t.Fatalf("got %d commands with DebugColliders, want 1", len(colliders))
	}
	if _, ok := colliders[0].(render.DrawCircle); !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", colliders[0])
	}

	velocities := world.DebugCommands(scene.DebugVelocities)
	if len(velocities) != 1 {
		t.Fatalf("got %d commands with DebugVelocities, want 1", len(velocities))
	}
	if _, ok := velocities[0].(render.DrawLine); !ok {
		t.Fatalf("got command type %T, want render.DrawLine", velocities[0])
	}

	both := world.DebugCommands(scene.DebugColliders | scene.DebugVelocities)
	if len(both) != 2 {
		t.Fatalf("got %d commands with both layers, want 2", len(both))
	}
}

func TestWorldDebugCommandsAppliesOverlayConfigOverrides(t *testing.T) {
	world := scene.NewWorld()

	bounds := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})

	commands := world.DebugCommands(
		scene.DebugGrid,
		scene.WithDebugBounds(bounds),
		scene.WithDebugCellSize(5),
	)

	if len(commands) == 0 {
		t.Fatalf("got 0 commands, want grid lines for a 10x10 bounds with cell size 5")
	}
}

func TestDebugLayersHasChecksFlagMembership(t *testing.T) {
	layers := scene.DebugColliders | scene.DebugGrid

	if !layers.Has(scene.DebugColliders) {
		t.Fatalf("expected layers to have DebugColliders")
	}
	if !layers.Has(scene.DebugGrid) {
		t.Fatalf("expected layers to have DebugGrid")
	}
	if layers.Has(scene.DebugContacts) {
		t.Fatalf("expected layers to not have DebugContacts")
	}
}

func TestWithDebugBoundsIgnoresInvalidRect(t *testing.T) {
	cfg := scene.DefaultDebugOverlayConfig()
	original := cfg.Bounds

	scene.WithDebugBounds(math2d.Rect{})(&cfg)

	if cfg.Bounds != original {
		t.Fatalf("got bounds %+v, want unchanged %+v", cfg.Bounds, original)
	}
}

func TestWithDebugCellSizeIgnoresNonPositiveValue(t *testing.T) {
	cfg := scene.DefaultDebugOverlayConfig()
	original := cfg.CellSize

	scene.WithDebugCellSize(0)(&cfg)
	scene.WithDebugCellSize(-1)(&cfg)

	if cfg.CellSize != original {
		t.Fatalf("got cell size %v, want unchanged %v", cfg.CellSize, original)
	}
}

func TestWithDebugOverlayConfigOverridesOnlyProvidedFields(t *testing.T) {
	cfg := scene.DefaultDebugOverlayConfig()
	wantStyle := cfg.Style

	bounds := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 20, Y: 20})
	scene.WithDebugOverlayConfig(scene.DebugOverlayConfig{
		Bounds:   bounds,
		CellSize: 4,
	})(&cfg)

	if cfg.Bounds != bounds {
		t.Fatalf("got bounds %+v, want %+v", cfg.Bounds, bounds)
	}
	if cfg.CellSize != 4 {
		t.Fatalf("got cell size %v, want 4", cfg.CellSize)
	}
	if cfg.Style != wantStyle {
		t.Fatalf("got style %+v, want unchanged default %+v", cfg.Style, wantStyle)
	}
}

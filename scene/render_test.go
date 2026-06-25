package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

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

func TestWorldDebugColliderCommandsCreatesCircleStrokeForCollider(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	position := math2d.Vec2{X: 4, Y: 7}
	color := render.RGBA(1, 1, 0, 1)

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 2,
	})

	commands := world.DebugColliderCommands(color, 0.1, 100)
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

	if command.Fill.A != 0 {
		t.Fatalf("got fill %+v, want zero-value fill", command.Fill)
	}

	if command.Stroke != color {
		t.Fatalf("got stroke %+v, want %+v", command.Stroke, color)
	}

	if command.StrokeWidth != 0.1 {
		t.Fatalf("got stroke width %v, want 0.1", command.StrokeWidth)
	}

	if command.Layer != 100 {
		t.Fatalf("got layer %d, want 100", command.Layer)
	}
}

func TestWorldDebugColliderCommandsSkipsColliderWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 2,
	})

	commands := world.DebugColliderCommands(render.RGBA(1, 1, 0, 1), 0.1, 100)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func TestWorldDebugContactCommandsCreatesPointAndNormalLineForOverlap(t *testing.T) {
	world := scene.NewWorld()
	color := render.RGBA(1, 0, 1, 1)

	first := world.Spawn()
	world.SetTransform(first, scene.Transform{
		Position: math2d.Vec2{X: 0, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(first, scene.CircleCollider{
		Radius: 2,
	})

	second := world.Spawn()
	world.SetTransform(second, scene.Transform{
		Position: math2d.Vec2{X: 3, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(second, scene.CircleCollider{
		Radius: 2,
	})

	commands := world.DebugContactCommands(color, 0.2, 0.1, 1, 101)
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}

	point, ok := commands[0].(render.DrawPoint)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawPoint", commands[0])
	}

	wantPoint := math2d.Vec2{X: 1.5, Y: 0}
	if point.Center != wantPoint {
		t.Fatalf("got point center %+v, want %+v", point.Center, wantPoint)
	}

	if point.Radius != 0.2 {
		t.Fatalf("got point radius %v, want 0.2", point.Radius)
	}

	if point.Color != color {
		t.Fatalf("got point color %+v, want %+v", point.Color, color)
	}

	if point.Layer != 101 {
		t.Fatalf("got point layer %d, want 101", point.Layer)
	}

	line, ok := commands[1].(render.DrawLine)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawLine", commands[1])
	}

	if line.From != wantPoint {
		t.Fatalf("got line from %+v, want %+v", line.From, wantPoint)
	}

	wantLineTo := math2d.Vec2{X: 2.5, Y: 0}
	if line.To != wantLineTo {
		t.Fatalf("got line to %+v, want %+v", line.To, wantLineTo)
	}

	if line.Width != 0.1 {
		t.Fatalf("got line width %v, want 0.1", line.Width)
	}

	if line.Color != color {
		t.Fatalf("got line color %+v, want %+v", line.Color, color)
	}

	if line.Layer != 101 {
		t.Fatalf("got line layer %d, want 101", line.Layer)
	}
}

func TestWorldDebugContactCommandsReturnsNoCommandsWithoutOverlap(t *testing.T) {
	world := scene.NewWorld()

	first := world.Spawn()
	world.SetTransform(first, scene.Transform{
		Position: math2d.Vec2{X: 0, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(first, scene.CircleCollider{
		Radius: 2,
	})

	second := world.Spawn()
	world.SetTransform(second, scene.Transform{
		Position: math2d.Vec2{X: 5, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(second, scene.CircleCollider{
		Radius: 2,
	})

	commands := world.DebugContactCommands(render.RGBA(1, 0, 1, 1), 0.2, 0.1, 1, 101)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

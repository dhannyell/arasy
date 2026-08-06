package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
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
		BaseSprite: scene.BaseSprite{
			Color: color,
			Layer: 5,
		},
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

	if command.Layer() != 5 {
		t.Fatalf("got layer %d, want 5", command.Layer())
	}
}

func TestWorldRenderCommandsSkipsEntityWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: 2,
		BaseSprite: scene.BaseSprite{
			Color: render.RGBA(1, 0, 0, 1),
			Layer: 5,
		},
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

	if command.Layer() != 100 {
		t.Fatalf("got layer %d, want 100", command.Layer())
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

func TestWorldDebugColliderCommandsSortsByEntityID(t *testing.T) {
	world := scene.NewWorld()
	first := world.Spawn()
	second := world.Spawn()

	world.SetTransform(second, scene.Transform{
		Position: math2d.Vec2{X: 20, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(second, scene.CircleCollider{Radius: 2})

	world.SetTransform(first, scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(first, scene.CircleCollider{Radius: 1})

	commands := world.DebugColliderCommands(render.RGBA(1, 1, 0, 1), 0.1, 100)
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}

	firstCommand, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}
	secondCommand, ok := commands[1].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[1])
	}
	if firstCommand.Radius != 1 || secondCommand.Radius != 2 {
		t.Fatalf("collider radii order = %v, %v; want 1, 2", firstCommand.Radius, secondCommand.Radius)
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

	world.ResolveCircleOverlaps()
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

	if point.Layer() != 101 {
		t.Fatalf("got point layer %d, want 101", point.Layer())
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

	if line.Layer() != 101 {
		t.Fatalf("got line layer %d, want 101", line.Layer())
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

	world.ResolveCircleOverlaps()
	commands := world.DebugContactCommands(render.RGBA(1, 0, 1, 1), 0.2, 0.1, 1, 101)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func TestWorldDebugGridCommandsReturnsNoCommandsWithoutGridSnapshot(t *testing.T) {
	world := scene.NewWorld()

	commands := world.DebugGridCommands(render.RGBA(0, 1, 1, 1), 0.2, 1, 200)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func TestWorldDebugGridCommandsCreatesDashedLinesForPopulatedCells(t *testing.T) {
	world := scene.NewWorld()
	color := render.RGBA(0, 1, 1, 1)

	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 2, Y: 2},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 1,
	})

	world.DetectCircleOverlapsGrid(4)
	commands := world.DebugGridCommands(color, 0.2, 8, 200)

	if len(commands) == 0 {
		t.Fatal("expected debug grid commands")
	}

	line, ok := commands[0].(render.DrawLine)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawLine", commands[0])
	}

	if line.Width != 0.2 {
		t.Fatalf("line width = %v, want 0.2", line.Width)
	}

	if line.Color != color {
		t.Fatalf("line color = %+v, want %+v", line.Color, color)
	}

	if line.Layer() != 200 {
		t.Fatalf("line layer = %v, want 200", line.Layer())
	}

	if line.From.X < 0 || line.From.X > 4 || line.From.Y < 0 || line.From.Y > 4 {
		t.Fatalf("line from %+v does not use the configured cell size", line.From)
	}

	if line.To.X < 0 || line.To.X > 4 || line.To.Y < 0 || line.To.Y > 4 {
		t.Fatalf("line to %+v does not use the configured cell size", line.To)
	}
}

func TestWorldDebugActiveGridCommandsCreatesFilledRectsForPopulatedCells(t *testing.T) {
	world := scene.NewWorld()
	fill := render.RGBA(1, 0.5, 0, 0.25)
	stroke := render.RGBA(1, 0.75, 0, 1)

	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 10},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: 1,
	})

	world.DetectCircleOverlapsGrid(8)
	commands := world.DebugActiveGridCommands(fill, stroke, 0.2, 201)

	if len(commands) == 0 {
		t.Fatal("expected active grid commands")
	}

	rect, ok := commands[0].(render.DrawRect)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawRect", commands[0])
	}

	if rect.Rect.Width() != 8 || rect.Rect.Height() != 8 {
		t.Fatalf("rect size = (%v, %v), want (8, 8)", rect.Rect.Width(), rect.Rect.Height())
	}

	if rect.Fill != fill {
		t.Fatalf("rect fill = %+v, want %+v", rect.Fill, fill)
	}

	if rect.Stroke != stroke {
		t.Fatalf("rect stroke = %+v, want %+v", rect.Stroke, stroke)
	}

	if rect.StrokeWidth != 0.2 {
		t.Fatalf("rect stroke width = %v, want 0.2", rect.StrokeWidth)
	}

	if rect.Layer() != 201 {
		t.Fatalf("rect layer = %v, want 201", rect.Layer())
	}
}

func TestWorldDebugCandidateGridCommandsCreatesFilledRectsOnlyForCandidateCells(t *testing.T) {
	world := scene.NewWorld()
	fill := render.RGBA(1, 1, 0, 0.35)
	stroke := render.RGBA(1, 1, 0, 1)

	first := world.Spawn()
	world.SetTransform(first, scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 2},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(first, scene.CircleCollider{
		Radius: 0.5,
	})

	second := world.Spawn()
	world.SetTransform(second, scene.Transform{
		Position: math2d.Vec2{X: 11, Y: 2},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(second, scene.CircleCollider{
		Radius: 0.5,
	})

	third := world.Spawn()
	world.SetTransform(third, scene.Transform{
		Position: math2d.Vec2{X: 2, Y: 2},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(third, scene.CircleCollider{
		Radius: 0.5,
	})

	world.DetectCircleOverlapsGrid(8)

	commands := world.DebugCandidateGridCommands(fill, stroke, 0.3, 202)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}

	rect, ok := commands[0].(render.DrawRect)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawRect", commands[0])
	}

	if rect.Rect.Min != (math2d.Vec2{X: 8, Y: 0}) {
		t.Fatalf("rect min = %+v, want {8 0}", rect.Rect.Min)
	}

	if rect.Rect.Width() != 8 || rect.Rect.Height() != 8 {
		t.Fatalf("rect size = (%v, %v), want (8, 8)", rect.Rect.Width(), rect.Rect.Height())
	}

	if rect.Fill != fill {
		t.Fatalf("rect fill = %+v, want %+v", rect.Fill, fill)
	}

	if rect.Stroke != stroke {
		t.Fatalf("rect stroke = %+v, want %+v", rect.Stroke, stroke)
	}

	if rect.StrokeWidth != 0.3 {
		t.Fatalf("rect stroke width = %v, want 0.3", rect.StrokeWidth)
	}

	if rect.Layer() != 202 {
		t.Fatalf("rect layer = %v, want 202", rect.Layer())
	}
}

func TestDebugGridAreaCommandsCreatesClippedGridInsideBounds(t *testing.T) {
	bounds := math2d.NewRect(
		math2d.Vec2{X: 5.25, Y: 3.4},
		math2d.Vec2{X: 105, Y: 68},
	)
	color := render.RGBA(0.5, 0.5, 0.5, 0.4)

	commands := scene.DebugGridAreaCommands(bounds, 8, color, 0.1, 1, 200)
	if len(commands) == 0 {
		t.Fatal("expected debug grid area commands")
	}

	for _, command := range commands {
		line, ok := command.(render.DrawLine)
		if !ok {
			t.Fatalf("got command type %T, want render.DrawLine", command)
		}

		if line.Width != 0.1 {
			t.Fatalf("line width = %v, want 0.1", line.Width)
		}

		if line.Color != color {
			t.Fatalf("line color = %+v, want %+v", line.Color, color)
		}

		if line.Layer() != 200 {
			t.Fatalf("line layer = %v, want 200", line.Layer())
		}

		assertPointInsideRect(t, line.From, bounds)
		assertPointInsideRect(t, line.To, bounds)
	}
}

func TestDebugGridAreaCommandsReturnsNoCommandsWithInvalidInput(t *testing.T) {
	bounds := math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 100, Y: 100},
	)

	commands := scene.DebugGridAreaCommands(bounds, 0, render.RGBA(1, 1, 1, 1), 0.1, 1, 200)
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0", len(commands))
	}
}

func assertPointInsideRect(t *testing.T, point math2d.Vec2, rect math2d.Rect) {
	t.Helper()

	if point.X < rect.Min.X || point.X > rect.Max.X || point.Y < rect.Min.Y || point.Y > rect.Max.Y {
		t.Fatalf("point %+v is outside rect %+v", point, rect)
	}
}

// spawnTorch spawns a still point light: an entity with only Transform and
// PointLight, the declared shape for a light that does not move.
func spawnTorch(w *scene.World, pos math2d.Vec2, l scene.PointLight) scene.EntityID {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: pos})
	w.SetPointLight(id, l)
	return id
}

// Distances and radii below are picked so every expected attenuation value
// is exact in binary, allowing == on colors. RenderCommands renders at
// alpha 0, so these cases also guard the non-interpolated path.
func TestRenderCommandsPointLightAttenuation(t *testing.T) {
	cases := []struct {
		name    string
		at      math2d.Vec2
		falloff float64
		want    float64 // modulation factor applied to a white sprite
	}{
		{"light center: full contribution", math2d.Vec2{}, 0, 1},
		// The curve is 1 - d²/R², not linear in d: DistanceSq keeps the
		// evaluation inside the plan's + - * / budget, sqrt-free.
		{"halfway: smooth quadratic curve", math2d.Vec2{X: 5}, 0, 0.75},
		{"radius edge: zero", math2d.Vec2{X: 10}, 0, 0},
		{"outside the radius: zero", math2d.Vec2{X: 20}, 0, 0},
		{"falloff squares the same curve", math2d.Vec2{X: 5}, 1, 0.5625},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			world := scene.NewWorld()
			world.SetLitLayers(0, 5)
			spawnTorch(world, math2d.Vec2{}, scene.PointLight{
				Color:   render.RGBA(1, 1, 1, 1),
				Radius:  10,
				Falloff: tc.falloff,
			})

			body := world.Spawn()
			world.SetTransform(body, scene.Transform{Position: tc.at})
			world.SetCircleSprite(body, scene.CircleSprite{
				Radius:     1,
				BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1), Layer: 5},
			})

			commands := world.RenderCommands()
			if len(commands) != 1 {
				t.Fatalf("got %d commands, want 1", len(commands))
			}
			circle, ok := commands[0].(render.DrawCircle)
			if !ok {
				t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
			}
			want := render.RGBA(tc.want, tc.want, tc.want, 1)
			if circle.Fill != want {
				t.Fatalf("got fill %+v, want %+v", circle.Fill, want)
			}
		})
	}
}

func TestRenderCommandsPointLightsSumAndClampPreservingAlpha(t *testing.T) {
	world := scene.NewWorld()
	world.SetLitLayers(0, 5)

	// Two 0.75-strength lights at the sprite sum to 1.5 and clamp to 1 per
	// channel. The authored alpha never joins the sum: light changes what a
	// sprite looks like, not how much of it exists.
	light := scene.PointLight{Color: render.RGBA(0.75, 0.75, 0.75, 1), Radius: 10}
	spawnTorch(world, math2d.Vec2{}, light)
	spawnTorch(world, math2d.Vec2{}, light)

	body := world.Spawn()
	world.SetTransform(body, scene.Transform{})
	world.SetCircleSprite(body, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 0.5), Layer: 5},
	})

	commands := world.RenderCommands()
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	circle, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}
	if want := render.RGBA(1, 1, 1, 0.5); circle.Fill != want {
		t.Fatalf("got fill %+v, want %+v", circle.Fill, want)
	}
}

func TestRenderCommandsLightingRespectsLitLayersBand(t *testing.T) {
	world := scene.NewWorld()
	world.SetLitLayers(0, 5)
	spawnTorch(world, math2d.Vec2{}, scene.PointLight{
		Color: render.RGBA(1, 1, 1, 1), Radius: 10,
	})

	authored := render.RGBA(0.25, 0.5, 0.75, 1)
	lit := world.Spawn()
	world.SetTransform(lit, scene.Transform{})
	world.SetCircleSprite(lit, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: authored, Layer: 5},
	})
	hud := world.Spawn()
	world.SetTransform(hud, scene.Transform{})
	world.SetCircleSprite(hud, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: authored, Layer: 6},
	})

	commands := world.RenderCommands()
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}
	// Layer sorts first, so the lit sprite (layer 5) precedes the HUD one.
	litCircle, okLit := commands[0].(render.DrawCircle)
	hudCircle, okHud := commands[1].(render.DrawCircle)
	if !okLit || !okHud {
		t.Fatalf("got command types %T and %T, want render.DrawCircle", commands[0], commands[1])
	}
	if litCircle.Fill != authored {
		t.Fatalf("in-band sprite at full light: got fill %+v, want authored %+v", litCircle.Fill, authored)
	}
	if hudCircle.Fill != authored {
		t.Fatalf("out-of-band sprite modulated: got fill %+v, want authored %+v", hudCircle.Fill, authored)
	}
}

func TestRenderCommandsLightingOffWithoutLitLayers(t *testing.T) {
	// Declared lights alone do nothing: SetLitLayers is the single opt-in.
	world := scene.NewWorld()
	spawnTorch(world, math2d.Vec2{}, scene.PointLight{
		Color: render.RGBA(1, 1, 1, 1), Radius: 10,
	})

	authored := render.RGBA(0.25, 0.5, 0.75, 1)
	body := world.Spawn()
	world.SetTransform(body, scene.Transform{})
	world.SetCircleSprite(body, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: authored, Layer: 5},
	})

	commands := world.RenderCommands()
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	circle, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}
	if circle.Fill != authored {
		t.Fatalf("got fill %+v, want authored %+v", circle.Fill, authored)
	}
}

func TestRenderCommandsAmbientLightModulatesPerChannel(t *testing.T) {
	world := scene.NewWorld()
	world.SetLitLayers(0, 5)
	world.SetAmbientLight(scene.AmbientLight{Color: render.RGBA(0.5, 0.25, 1, 1)})

	body := world.Spawn()
	world.SetTransform(body, scene.Transform{})
	world.SetCircleSprite(body, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1), Layer: 5},
	})

	commands := world.RenderCommands()
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	circle, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}
	if want := render.RGBA(0.5, 0.25, 1, 1); circle.Fill != want {
		t.Fatalf("got fill %+v, want %+v", circle.Fill, want)
	}
}

// The modulation is applied at three separate emit sites, one per sprite
// kind; a regression in one would be invisible to the circle-only tests.
func TestRenderCommandsLightingCoversAllSpriteKinds(t *testing.T) {
	world := scene.NewWorld()
	world.SetLitLayers(0, 5)
	spawnTorch(world, math2d.Vec2{}, scene.PointLight{
		Color: render.RGBA(1, 1, 1, 1), Radius: 10,
	})

	at := scene.Transform{Position: math2d.Vec2{X: 5}} // 0.75 modulation
	white := render.RGBA(1, 1, 1, 1)

	circleID := world.Spawn()
	world.SetTransform(circleID, at)
	world.SetCircleSprite(circleID, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: white, Layer: 5},
	})
	rectID := world.Spawn()
	world.SetTransform(rectID, at)
	world.SetRectSprite(rectID, scene.RectSprite{
		Rect:        math2d.NewRectFromCorners(math2d.Vec2{X: -1, Y: -1}, math2d.Vec2{X: 1, Y: 1}),
		Stroke:      white,
		StrokeWidth: 0.1,
		BaseSprite:  scene.BaseSprite{Color: white, Layer: 5},
	})
	imageID := world.Spawn()
	world.SetTransform(imageID, at)
	world.SetImageSprite(imageID, scene.ImageSprite{
		Texture:    1,
		Size:       math2d.Vec2{X: 2, Y: 2},
		BaseSprite: scene.BaseSprite{Color: white, Layer: 5},
	})

	want := render.RGBA(0.75, 0.75, 0.75, 1)
	commands := world.RenderCommands()
	if len(commands) != 3 {
		t.Fatalf("got %d commands, want 3", len(commands))
	}
	for _, c := range commands {
		switch cmd := c.(type) {
		case render.DrawCircle:
			if cmd.Fill != want {
				t.Fatalf("circle fill %+v, want %+v", cmd.Fill, want)
			}
		case render.DrawRect:
			if cmd.Fill != want {
				t.Fatalf("rect fill %+v, want %+v", cmd.Fill, want)
			}
			// The stroke is lit with the fill: an authored-color outline on a
			// modulated body would read as a rendering bug.
			if cmd.Stroke != want {
				t.Fatalf("rect stroke %+v, want %+v", cmd.Stroke, want)
			}
		case render.DrawImage:
			if cmd.Tint != want {
				t.Fatalf("image tint %+v, want %+v", cmd.Tint, want)
			}
		default:
			t.Fatalf("unexpected command type %T", c)
		}
	}
}

func TestRenderCommandsWithLightingIdenticalAcrossClones(t *testing.T) {
	world := scene.NewWorld()
	world.SetLitLayers(0, 5)

	torch := spawnTorch(world, math2d.Vec2{}, scene.PointLight{
		Color: render.RGBA(1, 0.5, 0.25, 1), Radius: 10, Falloff: 1,
	})
	world.SetVelocity(torch, scene.Velocity{Linear: math2d.Vec2{X: 3, Y: 1}})

	body := world.Spawn()
	world.SetTransform(body, scene.Transform{Position: math2d.Vec2{X: 4, Y: 2}})
	world.SetCircleSprite(body, scene.CircleSprite{
		Radius:     1,
		BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1), Layer: 5},
	})
	world.SetVelocity(body, scene.Velocity{Linear: math2d.Vec2{X: -2, Y: 1}})

	world.SyncRenderState()
	world.Step(1, scene.StepConfig{})

	a := world.Clone().RenderCommandsInterpolated(0.5)
	b := world.RenderCommandsInterpolated(0.5)
	if len(a) != len(b) {
		t.Fatalf("command count differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("command %d differs:\nclone: %+v\nworld: %+v", i, a[i], b[i])
		}
	}
}

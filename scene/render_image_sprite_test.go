package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

func TestRenderCommandsCreatesScaledImageSprite(t *testing.T) {
	w := scene.NewWorld()
	id := w.Spawn()
	transform := scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 20},
		Rotation: 0.75,
		Scale:    math2d.Vec2{X: 2, Y: 3},
	}
	sprite := testImageSprite()
	w.SetTransform(id, transform)
	w.SetImageSprite(id, sprite)

	commands := w.RenderCommandsInterpolated(0)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	got, ok := commands[0].(render.DrawImage)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawImage", commands[0])
	}
	wantDest := math2d.NewRectFromCorners(
		math2d.Vec2{X: 2, Y: 11},
		math2d.Vec2{X: 18, Y: 29},
	)
	if got.Dest != wantDest {
		t.Fatalf("Dest = %+v, want %+v", got.Dest, wantDest)
	}
	if got.Texture != sprite.Texture || got.Src != sprite.Src || got.Tint != sprite.Tint || got.Rotation != transform.Rotation || got.Layer() != sprite.Layer {
		t.Fatalf("DrawImage did not preserve sprite/transform fields: %+v", got)
	}
}

func TestRenderCommandsSortsCircleAndImageByEntityID(t *testing.T) {
	w := scene.NewWorld()
	first := w.Spawn()
	second := w.Spawn()
	unit := math2d.Vec2{X: 1, Y: 1}

	w.SetTransform(second, scene.Transform{Position: math2d.Vec2{X: 20}, Scale: unit})
	w.SetImageSprite(second, testImageSprite())
	w.SetTransform(first, scene.Transform{Position: math2d.Vec2{X: 10}, Scale: unit})
	w.SetCircleSprite(first, scene.CircleSprite{Radius: 2})

	commands := w.RenderCommands()
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}
	if _, ok := commands[0].(render.DrawCircle); !ok {
		t.Fatalf("first command type = %T, want render.DrawCircle", commands[0])
	}
	if _, ok := commands[1].(render.DrawImage); !ok {
		t.Fatalf("second command type = %T, want render.DrawImage", commands[1])
	}
}

func TestRenderCommandsInterpolatesImageSpritePosition(t *testing.T) {
	w := scene.NewWorld()
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetImageSprite(id, scene.ImageSprite{Size: math2d.Vec2{X: 4, Y: 2}})
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 6}})
	w.SyncRenderState()
	w.Step(1, scene.StepConfig{})

	commands := w.RenderCommandsInterpolated(0.5)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	image := commands[0].(render.DrawImage)
	if got, want := image.Dest.Center(), (math2d.Vec2{X: 5, Y: 3}); got != want {
		t.Fatalf("interpolated Dest center = %+v, want %+v", got, want)
	}
}

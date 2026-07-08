package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

func TestRenderCommandsInterpolatedAtHalfAlphaPositionsCircleInMiddle(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 0, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: 2,
		BaseSprite: scene.BaseSprite{
			Color: render.RGBA(1, 0, 0, 1),
			Layer: 5,
		},
	})
	world.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 10}})

	world.SyncRenderState()
	world.Step(1, scene.StepConfig{})

	commands := world.RenderCommandsInterpolated(0.5)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}

	circle, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}

	want := math2d.Vec2{X: 5, Y: 5}
	if circle.Center != want {
		t.Fatalf("got center %+v, want %+v", circle.Center, want)
	}
}

func TestRenderCommandsInterpolatedZeroAlphaUsesCurrentPose(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()
	pos := math2d.Vec2{X: 7, Y: 3}
	world.SetTransform(entity, scene.Transform{Position: pos, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(entity, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 1, 0, 1), Layer: 1}})
	commands := world.RenderCommandsInterpolated(0)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	circle, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawCircle", commands[0])
	}
	if circle.Center != pos {
		t.Fatalf("got center %+v, want %+v", circle.Center, pos)
	}
}

func TestRenderCommandsInterpolatedWithoutPrevPoseUsesCurrent(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()
	pos := math2d.Vec2{X: 3, Y: 8}
	world.SetTransform(entity, scene.Transform{Position: pos, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(entity, scene.CircleSprite{Radius: 1.5, BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 1, 1), Layer: 2}})
	commands := world.RenderCommandsInterpolated(0.5)
	if len(commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(commands))
	}
	circle, ok := commands[0].(render.DrawCircle)
	if !ok {
		t.Fatalf("got command type %T", commands[0])
	}
	if circle.Center != pos {
		t.Fatalf("got center %+v, want %+v", circle.Center, pos)
	}
}

func TestSetTransformSyncsRenderStateOnTeleport(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Position: math2d.Vec2{X: 0, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SyncRenderState()
	world.SetTransform(entity, scene.Transform{Position: math2d.Vec2{X: 100, Y: 100}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(entity, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 0, 1), Layer: 1}})
	commands := world.RenderCommandsInterpolated(0.5)
	if len(commands) != 1 {
		t.Fatalf("got %d", len(commands))
	}
	circle := commands[0].(render.DrawCircle)
	want := math2d.Vec2{X: 100, Y: 100}
	if circle.Center != want {
		t.Fatalf("got center %+v, want %+v", circle.Center, want)
	}
}

func TestClonePreservesRenderTransforms(t *testing.T) {
	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 1, Y: 2}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(id, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 0, 0, 1), Layer: 5}})
	world.SyncRenderState()
	world.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 20}})
	world.Step(1, scene.StepConfig{})
	clone := world.Clone()
	cloneCmds := clone.RenderCommandsInterpolated(0.5)
	if len(cloneCmds) != 1 {
		t.Fatalf("got %d cmds", len(cloneCmds))
	}
	cloneCircle := cloneCmds[0].(render.DrawCircle)
	want := math2d.Vec2{X: 6, Y: 12}
	if cloneCircle.Center != want {
		t.Fatalf("clone center = %+v, want %+v", cloneCircle.Center, want)
	}
	origCmds := world.RenderCommandsInterpolated(0.5)
	origCircle := origCmds[0].(render.DrawCircle)
	if origCircle.Center != want {
		t.Fatalf("orig center = %+v", origCircle.Center)
	}
}

func TestDespawnCleansUpRenderTransforms(t *testing.T) {
	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 5, Y: 5}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(id, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1), Layer: 1}})
	world.SyncRenderState()
	world.Despawn(id)
	if len(world.RenderCommands()) != 0 {
		t.Fatal("expected no commands")
	}
	world.SyncRenderState()
	if len(world.RenderCommandsInterpolated(0.5)) != 0 {
		t.Fatal("expected no interpolated commands")
	}
}

func TestRenderCommandsEqInterpolatedZeroAlpha(t *testing.T) {
	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 4, Y: 7}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(id, scene.CircleSprite{Radius: 2, BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 0, 0, 1), Layer: 5}})
	legacy := world.RenderCommands()
	interp := world.RenderCommandsInterpolated(0)
	if len(legacy) != len(interp) {
		t.Fatalf("len %d vs %d", len(legacy), len(interp))
	}
	for i := range legacy {
		if legacy[i] != interp[i] {
			t.Fatalf("cmd %d differs", i)
		}
	}
}

package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestRenderCommandsIntoMatchesInterpolatedAndReusesBuffer(t *testing.T) {
	world := scene.NewWorld()
	for i := range 3 {
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: float64(i)}, Scale: math2d.Vec2{X: 1, Y: 1}})
		switch i {
		case 0:
			world.SetCircleSprite(id, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Layer: 2}})
		case 1:
			world.SetRectSprite(id, scene.RectSprite{Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 3}), BaseSprite: scene.BaseSprite{Layer: 1}})
		case 2:
			world.SetImageSprite(id, scene.ImageSprite{Texture: 1, Size: math2d.Vec2{X: 2, Y: 2}, Layer: 3})
		}
	}

	want := world.RenderCommandsInterpolated(0.5)
	buffer := make([]render.Command, 0, len(want))
	got := world.RenderCommandsInto(buffer, 0.5)
	assertRenderCommandsEqual(t, got, want)

	got = world.RenderCommandsInto(got[:0], 0.5)
	assertRenderCommandsEqual(t, got, want)
}

func TestWorldViewRenderCommandsInto(t *testing.T) {
	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(id, scene.CircleSprite{Radius: 1})

	commands := world.View().RenderCommandsInto(nil, 0)
	if len(commands) != 1 {
		t.Fatalf("RenderCommandsInto returned %d commands, want 1", len(commands))
	}
}

func TestCloneRenderTransformsRemainIndependent(t *testing.T) {
	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(id, scene.CircleSprite{Radius: 1})
	world.SyncRenderState()
	world.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 10}})
	world.Step(1, scene.StepConfig{})

	clone := world.Clone()
	world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 20}, Scale: math2d.Vec2{X: 1, Y: 1}})

	commands := clone.RenderCommandsInterpolated(0.5)
	circle := commands[0].(render.DrawCircle)
	if circle.Center != (math2d.Vec2{X: 5}) {
		t.Fatalf("clone interpolated center = %+v, want {5 0}", circle.Center)
	}
}

func assertRenderCommandsEqual(t *testing.T, got, want []render.Command) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("command count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

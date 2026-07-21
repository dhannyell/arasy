package scene_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

func TestRenderProjectionLiftsAndEmitsShadow(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(render.Projection{
		LiftPerZ:    1,
		ScalePerZ:   0.1,
		ShadowScale: math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha: 0.5,
	})
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 10, Y: 20}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetCircleSprite(id, scene.CircleSprite{
		Radius:     2,
		BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(1, 1, 1, 1)},
	})
	w.SetHeight(id, scene.Height{Z: 5})

	cmds := w.RenderCommands()
	if len(cmds) != 2 {
		t.Fatalf("got %d commands, want 2 (shadow + body)", len(cmds))
	}

	shadow, ok := cmds[0].(render.DrawEllipse)
	if !ok {
		t.Fatalf("cmds[0] = %T, want render.DrawEllipse drawn first (behind body)", cmds[0])
	}
	if shadow.Center != (math2d.Vec2{X: 10, Y: 20}) {
		t.Errorf("shadow center = %+v, want ground position (10,20)", shadow.Center)
	}
	if shadow.RadiusX != 2 || shadow.RadiusY != 2 {
		t.Errorf("shadow radii = (%v,%v), want (2,2)", shadow.RadiusX, shadow.RadiusY)
	}
	if shadow.Fill.A != 0.5 {
		t.Errorf("shadow alpha = %v, want 0.5", shadow.Fill.A)
	}

	body, ok := cmds[1].(render.DrawCircle)
	if !ok {
		t.Fatalf("cmds[1] = %T, want render.DrawCircle", cmds[1])
	}
	// Lifted by z*LiftPerZ = 5, scaled by 1 + z*ScalePerZ = 1.5.
	if body.Center.Y != 15 {
		t.Errorf("body Y = %v, want 15 (20 - 5)", body.Center.Y)
	}
	if body.Radius != 3 {
		t.Errorf("body radius = %v, want 3 (2 * 1.5)", body.Radius)
	}
}

func TestRenderZeroProjectionIgnoresHeight(t *testing.T) {
	// A world with no projection must render identically whether or not entities
	// carry Height — proof that 2D worlds are byte-for-byte unchanged.
	build := func(withHeight bool) []render.Command {
		w := scene.NewWorld()
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 4, Y: 7}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetCircleSprite(id, scene.CircleSprite{
			Radius:     2,
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(1, 0, 0, 1)},
		})
		if withHeight {
			w.SetHeight(id, scene.Height{Z: 9, VZ: 3})
		}
		return w.RenderCommands()
	}

	if !reflect.DeepEqual(build(false), build(true)) {
		t.Error("Height changed render output under a zero (identity) projection")
	}
}

func TestRenderDepthSortByY(t *testing.T) {
	w := scene.NewWorld()
	ys := []float64{30, 10, 20}
	for _, y := range ys {
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 0, Y: y}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetCircleSprite(id, scene.CircleSprite{
			Radius:     1,
			BaseSprite: scene.BaseSprite{Layer: 1},
		})
	}

	cmds := w.RenderCommands()
	if len(cmds) != 3 {
		t.Fatalf("got %d commands, want 3", len(cmds))
	}
	want := []float64{10, 20, 30} // ascending Y = back to front
	for i, c := range cmds {
		circle := c.(render.DrawCircle)
		if circle.Center.Y != want[i] {
			t.Errorf("command %d Y = %v, want %v (depth sorted by Y)", i, circle.Center.Y, want[i])
		}
	}
}

func TestRenderInterpolatesHeight(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(render.Projection{LiftPerZ: 1})
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 0, Y: 100}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetCircleSprite(id, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Layer: 0}})
	w.SetHeight(id, scene.Height{Z: 10, VZ: -60}) // -60/60 = -1 per tick

	w.SyncRenderState() // previous-frame height = 10

	config := scene.StepConfig{
		Bounds: math2d.Rect{Min: math2d.Vec2{X: -100, Y: -100}, Max: math2d.Vec2{X: 100, Y: 200}},
		Bounce: scene.BounceConfig{OpenSides: scene.SideLeft | scene.SideRight | scene.SideTop | scene.SideBottom},
	}
	w.Step(1.0/60.0, config) // current height Z = 9

	if cur, _ := w.Height(id); math.Abs(cur.Z-9) > 1e-9 {
		t.Fatalf("precondition: current Z = %v, want 9", cur.Z)
	}

	// alpha 0.5 interpolates z = lerp(10, 9, 0.5) = 9.5 → body Y = 100 - 9.5.
	cmds := w.RenderCommandsInterpolated(0.5)
	body := cmds[0].(render.DrawCircle)
	if math.Abs(body.Center.Y-90.5) > 1e-9 {
		t.Errorf("interpolated body Y = %v, want 90.5 (height interpolated, not stepped)", body.Center.Y)
	}
}

func TestRenderCompositeGroupNotInterleaved(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(render.Projection{
		LiftPerZ:    1,
		ShadowScale: math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha: 0.5,
	})
	for _, y := range []float64{10, 20} {
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 0, Y: y}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetCircleSprite(id, scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Layer: 1}})
		w.SetHeight(id, scene.Height{Z: 3})
	}

	cmds := w.RenderCommands()
	if len(cmds) != 4 {
		t.Fatalf("got %d commands, want 4 (shadow+body per entity)", len(cmds))
	}
	// Each entity's shadow must immediately precede its body — no interleaving.
	wantEllipse := []bool{true, false, true, false}
	for i, c := range cmds {
		_, isEllipse := c.(render.DrawEllipse)
		if isEllipse != wantEllipse[i] {
			t.Errorf("command %d isEllipse = %v, want %v (composite group interleaved)", i, isEllipse, wantEllipse[i])
		}
	}
}

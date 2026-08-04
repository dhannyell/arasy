package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// isoWorld is a world under the classic 2:1 view with depth growing along
// +X+Y, the setup every isometric game shares.
func isoWorld() *scene.World {
	w := scene.NewWorld()
	w.SetProjection(render.GroundProjection(render.Camera{
		World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		View:  math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5},
	}, render.ProjectionLight{}))
	w.SetSortAxis(math2d.Vec2{X: 1, Y: 1})
	return w
}

// A CommandSprite exists so that a raw-command prop can be walked around: the
// same unit must draw behind it from one side and in front of it from the
// other. Without the component that ordering needed hand-managed layers.
func TestCommandSpriteSortsAgainstEntities(t *testing.T) {
	w := isoWorld()

	prop := w.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: math2d.Vec2{X: 10, Y: 10}},
		CommandSprite: &scene.CommandSprite{
			Layer:    5,
			Commands: []render.Command{render.DrawRect{Fill: render.RGBA(1, 0, 0, 1)}},
		},
	})
	_ = prop

	unit := w.SpawnBundle(scene.Bundle{
		Transform:    &scene.Transform{Position: math2d.Vec2{X: 4, Y: 4}},
		CircleSprite: &scene.CircleSprite{Radius: 1, BaseSprite: scene.BaseSprite{Layer: 5}},
	})

	indexOf := func(cmds []render.Command, matchCircle bool) int {
		for i, c := range cmds {
			_, isCircle := c.(render.DrawCircle)
			if isCircle == matchCircle {
				return i
			}
		}
		t.Fatal("expected command kind not emitted")
		return -1
	}

	// Behind the prop along +X+Y: the unit paints first.
	cmds := w.RenderCommands()
	if indexOf(cmds, true) > indexOf(cmds, false) {
		t.Fatal("unit behind the prop drew in front of it")
	}

	// Walk the unit past the prop: now it paints after.
	w.SetTransform(unit, scene.Transform{Position: math2d.Vec2{X: 16, Y: 16}, Scale: math2d.Vec2{X: 1, Y: 1}})
	cmds = w.RenderCommands()
	if indexOf(cmds, true) < indexOf(cmds, false) {
		t.Fatal("unit in front of the prop drew behind it")
	}

	// The prop's command draws on the sprite's layer, not the one the raw
	// command happened to be authored with.
	if got := cmds[indexOf(cmds, false)].Layer(); got != 5 {
		t.Fatalf("command sprite emitted layer %d, want the component's 5", got)
	}
}

// An attachment borrows everything positional from its parent: it hangs its
// offset up the lift axis, rides the parent's height, and paints right after
// the body it decorates -- one composite thing in the painter order.
func TestAttachmentRidesItsParent(t *testing.T) {
	w := isoWorld()
	lift := w.Projection().LiftVector() // (-1, -1) under the 2:1 view

	foot := math2d.Vec2{X: 20, Y: 20}
	trunk := w.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: foot},
		RectSprite: &scene.RectSprite{
			Rect:       math2d.NewRect(foot.Sub(math2d.Vec2{X: 1, Y: 3}), math2d.Vec2{X: 2, Y: 3}),
			BaseSprite: scene.BaseSprite{Layer: 5},
		},
	})
	w.SpawnAttached(trunk, math2d.Vec2{}, 4, scene.Bundle{
		CircleSprite: &scene.CircleSprite{Radius: 2, BaseSprite: scene.BaseSprite{Layer: 5}},
	})

	canopyOf := func(cmds []render.Command) render.DrawCircle {
		for _, c := range cmds {
			if circle, ok := c.(render.DrawCircle); ok {
				return circle
			}
		}
		t.Fatal("canopy circle not emitted")
		return render.DrawCircle{}
	}

	cmds := w.RenderCommands()
	if want := foot.Add(lift.Mul(4)); canopyOf(cmds).Center != want {
		t.Fatalf("canopy center = %v, want the foot four lift units up: %v", canopyOf(cmds).Center, want)
	}
	if _, isRect := cmds[0].(render.DrawRect); !isRect {
		t.Fatal("the trunk should paint before the canopy attached to it")
	}

	// When the parent jumps, the whole composite jumps: the canopy adds the
	// parent's height under its own offset without being told.
	w.SetHeight(trunk, scene.Height{Z: 3})
	want := foot.Add(lift.Mul(4 + 3))
	if got := canopyOf(w.RenderCommands()).Center; got.Sub(want).Len() > 1e-9 {
		t.Fatalf("canopy with parent at Z=3 sits at %v, want %v", got, want)
	}
}

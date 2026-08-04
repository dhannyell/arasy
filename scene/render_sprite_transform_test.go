package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func spawnCircle(w *scene.World, pos math2d.Vec2) scene.EntityID {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: pos, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetCircleSprite(id, scene.CircleSprite{
		Radius:     2,
		BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(1, 1, 1, 1)},
	})
	return id
}

func vecNear(a, b math2d.Vec2, epsilon float64) bool {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx < epsilon && dy < epsilon
}

func TestRenderShadowCarriesStretchTransform(t *testing.T) {
	build := func(stretch float64) render.DrawEllipse {
		w := scene.NewWorld()
		w.SetProjection(render.Projection{
			LiftPerZ:         1,
			ShadowScale:      math2d.Vec2{X: 1, Y: 1},
			ShadowAlpha:      0.5,
			ShadowEscapeAxis: math2d.Vec2{X: 1},
			ShadowEscapePerZ: 0.3,
			ShadowStretch:    stretch,
		})
		id := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
		w.SetHeight(id, scene.Height{Z: 5})
		return w.RenderCommands()[0].(render.DrawEllipse)
	}

	stretched := build(2)
	if stretched.Transform.IsIdentity() {
		t.Error("shadow Transform is identity with ShadowStretch != 1")
	}
	// The ellipse stretches about its own centre, so the centre is the fixed
	// point and the command's Center is already where the mark lies.
	if got := stretched.Transform.Apply(stretched.Center); !vecNear(got, stretched.Center, 1e-9) {
		t.Errorf("shadow centre moved: Apply(%v) = %v", stretched.Center, got)
	}

	if round := build(0); !round.Transform.IsIdentity() {
		t.Error("shadow Transform not identity with ShadowStretch = 0")
	}
}

func TestRenderSpriteTransformAppliesToBody2D(t *testing.T) {
	// Pure 2D world: no projection, no Height. The per-entity affine must
	// still reach the body command.
	w := scene.NewWorld()
	center := math2d.Vec2{X: 10, Y: 20}
	id := spawnCircle(w, center)
	w.SetSpriteTransform(id, scene.SpriteTransform{M: math2d.Scale(math2d.Vec2{X: 2, Y: 0.5})})

	body := w.RenderCommands()[0].(render.DrawCircle)
	if body.Transform.IsIdentity() {
		t.Fatal("body Transform is identity despite SpriteTransform")
	}
	if got := body.Transform.Apply(center); !vecNear(got, center, 1e-9) {
		t.Errorf("Apply(center) = %v, want fixed pivot %v (deform about the body center)", got, center)
	}
	edge := math2d.Vec2{X: center.X + 1, Y: center.Y}
	if got, want := body.Transform.Apply(edge), (math2d.Vec2{X: center.X + 2, Y: center.Y}); !vecNear(got, want, 1e-9) {
		t.Errorf("Apply(%v) = %v, want %v (x stretched 2x about the center)", edge, got, want)
	}
}

func TestRenderSpriteTransformAppliesAtGroundLevel(t *testing.T) {
	// Regression: with a projection set but z == 0 the body must still carry
	// the affine — squash on bounce happens at ground level.
	w := scene.NewWorld()
	w.SetProjection(render.Projection{LiftPerZ: 1})
	center := math2d.Vec2{X: 10, Y: 20}
	id := spawnCircle(w, center)
	w.SetHeight(id, scene.Height{Z: 0})
	w.SetSpriteTransform(id, scene.SpriteTransform{M: math2d.Scale(math2d.Vec2{X: 2, Y: 0.5})})

	cmds := w.RenderCommands()
	if len(cmds) != 1 {
		t.Fatalf("got %d commands, want 1 (no shadow at z=0)", len(cmds))
	}
	body := cmds[0].(render.DrawCircle)
	if body.Transform.IsIdentity() {
		t.Error("body Transform is identity at z=0 despite SpriteTransform")
	}
}

func TestRenderSpriteTransformAppliesAboutLiftedCenter(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(render.Projection{LiftPerZ: 1})
	ground := math2d.Vec2{X: 10, Y: 20}
	id := spawnCircle(w, ground)
	w.SetHeight(id, scene.Height{Z: 5})
	w.SetSpriteTransform(id, scene.SpriteTransform{M: math2d.Scale(math2d.Vec2{X: 2, Y: 0.5})})

	cmds := w.RenderCommands()
	body := cmds[len(cmds)-1].(render.DrawCircle)
	lifted := math2d.Vec2{X: 10, Y: 15} // 20 - 5*LiftPerZ
	if got := body.Transform.Apply(lifted); !vecNear(got, lifted, 1e-9) {
		t.Errorf("Apply(lifted center) = %v, want fixed pivot %v", got, lifted)
	}
	if got := body.Transform.Apply(ground); vecNear(got, ground, 1e-9) {
		t.Error("ground point is fixed: transform must pivot on the lifted center, not the ground")
	}
}

func TestRenderWithoutSpriteTransformIsIdentity(t *testing.T) {
	// Retrocompat: entities without the component render with an identity
	// Transform on every body command shape.
	w := scene.NewWorld()
	circle := spawnCircle(w, math2d.Vec2{X: 0, Y: 0})
	rect := w.Spawn()
	w.SetTransform(rect, scene.Transform{Position: math2d.Vec2{X: 50, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetRectSprite(rect, scene.RectSprite{
		Rect:       math2d.Rect{Min: math2d.Vec2{X: -2, Y: -2}, Max: math2d.Vec2{X: 2, Y: 2}},
		BaseSprite: scene.BaseSprite{Layer: 1},
	})
	_ = circle

	for i, c := range w.RenderCommands() {
		var tr math2d.Affine
		switch cmd := c.(type) {
		case render.DrawCircle:
			tr = cmd.Transform
		case render.DrawRect:
			tr = cmd.Transform
		default:
			t.Fatalf("command %d = %T, want DrawCircle/DrawRect", i, c)
		}
		if !tr.IsIdentity() {
			t.Errorf("command %d (%T) Transform not identity without SpriteTransform", i, c)
		}
	}
}

func TestSpriteTransformCloneAndDespawn(t *testing.T) {
	w := scene.NewWorld()
	id := spawnCircle(w, math2d.Vec2{X: 1, Y: 2})
	want := scene.SpriteTransform{M: math2d.Shear(0.5, 0)}
	w.SetSpriteTransform(id, want)

	snap := w.Clone()
	got, ok := snap.SpriteTransform(id)
	if !ok || got != want {
		t.Fatalf("Clone lost SpriteTransform: got %+v, ok=%v, want %+v", got, ok, want)
	}

	w.Despawn(id)
	if _, ok := w.SpriteTransform(id); ok {
		t.Error("SpriteTransform still present after Despawn")
	}
	if _, ok := snap.SpriteTransform(id); !ok {
		t.Error("snapshot lost SpriteTransform after despawning the source")
	}
}

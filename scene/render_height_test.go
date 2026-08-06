package scene_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
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

// shadowProjection is a plain overhead-light projection: full-footprint
// shadow, no escape, no stretch, so every assertion reads directly off the
// style arithmetic.
func shadowProjection() render.Projection {
	return render.Projection{
		LiftPerZ:    1,
		ShadowScale: math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha: 0.5,
	}
}

func firstEllipse(t *testing.T, commands []render.Command) render.DrawEllipse {
	t.Helper()
	for _, c := range commands {
		if e, ok := c.(render.DrawEllipse); ok {
			return e
		}
	}
	t.Fatal("no DrawEllipse emitted")
	return render.DrawEllipse{}
}

func countEllipses(commands []render.Command) int {
	n := 0
	for _, c := range commands {
		if _, ok := c.(render.DrawEllipse); ok {
			n++
		}
	}
	return n
}

// The contact shadow is opt-in by the component: a grounded entity without a
// ShadowStyle emits exactly what it emits today, byte for byte.
func TestShadowStyleContactIsOptIn(t *testing.T) {
	build := func(withStyle bool) []render.Command {
		w := scene.NewWorld()
		w.SetProjection(shadowProjection())
		id := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
		w.SetHeight(id, scene.Height{Z: 0})
		if withStyle {
			w.SetShadowStyle(id, scene.ShadowStyle{})
		}
		return w.RenderCommands()
	}

	without := build(false)
	if countEllipses(without) != 0 {
		t.Fatal("grounded entity without ShadowStyle emitted a shadow")
	}

	with := build(true)
	if countEllipses(with) != 1 {
		t.Fatalf("grounded entity with ShadowStyle emitted %d shadows, want 1", countEllipses(with))
	}

	// The contact shadow is the footprint at the projection's full alpha.
	shadow := firstEllipse(t, with)
	if shadow.RadiusX != 2 || shadow.RadiusY != 2 {
		t.Errorf("contact radii = (%v, %v), want the footprint (2, 2)", shadow.RadiusX, shadow.RadiusY)
	}
	if shadow.Fill.A != 0.5 {
		t.Errorf("contact alpha = %v, want the projection's 0.5", shadow.Fill.A)
	}
}

// The zero value keeps the projection's defaults: an airborne entity draws
// the same frame with and without a zero ShadowStyle.
func TestShadowStyleZeroValueIsNeutral(t *testing.T) {
	build := func(withStyle bool) []render.Command {
		w := scene.NewWorld()
		w.SetProjection(shadowProjection())
		id := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
		w.SetHeight(id, scene.Height{Z: 3})
		if withStyle {
			w.SetShadowStyle(id, scene.ShadowStyle{})
		}
		return w.RenderCommands()
	}

	if !reflect.DeepEqual(build(false), build(true)) {
		t.Error("zero ShadowStyle changed an airborne entity's commands")
	}
}

func TestShadowStyleAdjustsTheEmission(t *testing.T) {
	baseline := func() render.DrawEllipse {
		w := scene.NewWorld()
		w.SetProjection(shadowProjection())
		id := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
		w.SetHeight(id, scene.Height{Z: 0})
		w.SetShadowStyle(id, scene.ShadowStyle{})
		return firstEllipse(t, w.RenderCommands())
	}()

	build := func(style scene.ShadowStyle) []render.Command {
		w := scene.NewWorld()
		w.SetProjection(shadowProjection())
		id := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
		w.SetHeight(id, scene.Height{Z: 0})
		w.SetShadowStyle(id, style)
		return w.RenderCommands()
	}

	t.Run("Scale", func(t *testing.T) {
		shadow := firstEllipse(t, build(scene.ShadowStyle{Scale: 2}))
		if shadow.RadiusX != baseline.RadiusX*2 || shadow.RadiusY != baseline.RadiusY*2 {
			t.Errorf("radii = (%v, %v), want the baseline (%v, %v) doubled",
				shadow.RadiusX, shadow.RadiusY, baseline.RadiusX, baseline.RadiusY)
		}
	})

	t.Run("Alpha", func(t *testing.T) {
		shadow := firstEllipse(t, build(scene.ShadowStyle{Alpha: 0.5}))
		if shadow.Fill.A != baseline.Fill.A*0.5 {
			t.Errorf("alpha = %v, want half the baseline %v", shadow.Fill.A, baseline.Fill.A)
		}
	})

	t.Run("Offset", func(t *testing.T) {
		offset := math2d.Vec2{X: 1.5, Y: -0.5}
		shadow := firstEllipse(t, build(scene.ShadowStyle{Offset: offset}))
		if shadow.Center != baseline.Center.Add(offset) {
			t.Errorf("centre = %v, want the baseline %v nudged by %v",
				shadow.Center, baseline.Center, offset)
		}
	})

	t.Run("Disable", func(t *testing.T) {
		if n := countEllipses(build(scene.ShadowStyle{Disable: true})); n != 0 {
			t.Errorf("disabled style still emitted %d shadows", n)
		}
	})
}

// Disable removes the shadow but never grounds the body: the lift and scale
// of an airborne entity pass through untouched.
func TestShadowStyleDisableKeepsTheBodyLifted(t *testing.T) {
	build := func(style *scene.ShadowStyle) []render.Command {
		w := scene.NewWorld()
		w.SetProjection(shadowProjection())
		id := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
		w.SetHeight(id, scene.Height{Z: 4})
		if style != nil {
			w.SetShadowStyle(id, *style)
		}
		return w.RenderCommands()
	}

	plain := build(nil)
	disabled := build(&scene.ShadowStyle{Disable: true})

	if countEllipses(disabled) != 0 {
		t.Fatal("Disable left the airborne shadow in place")
	}

	var wantBody, gotBody render.DrawCircle
	for _, c := range plain {
		if b, ok := c.(render.DrawCircle); ok {
			wantBody = b
		}
	}
	for _, c := range disabled {
		if b, ok := c.(render.DrawCircle); ok {
			gotBody = b
		}
	}
	if gotBody.Center != wantBody.Center || gotBody.Radius != wantBody.Radius {
		t.Errorf("body changed under Disable: got %+v, want %+v", gotBody, wantBody)
	}
}

// The contact shadow of an ImageSprite derives from the authored
// ShadowFootprint, exactly as the airborne shadow does.
func TestShadowStyleContactUsesImageFootprint(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(shadowProjection())
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 5, Y: 5}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetImageSprite(id, scene.ImageSprite{
		Size:            math2d.Vec2{X: 12, Y: 6},
		ShadowFootprint: math2d.Vec2{X: 1.8, Y: 0.8},
	})
	w.SetHeight(id, scene.Height{Z: 0})
	w.SetShadowStyle(id, scene.ShadowStyle{})

	shadow := firstEllipse(t, w.RenderCommands())
	if shadow.RadiusX != 1.8 || shadow.RadiusY != 0.8 {
		t.Errorf("radii = (%v, %v), want the authored footprint (1.8, 0.8)",
			shadow.RadiusX, shadow.RadiusY)
	}
}

// An attachment never casts a shadow of its own: the parent's ShadowStyle
// styles the one shadow the composite has.
func TestShadowStyleAttachmentKeepsOneShadow(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(shadowProjection())
	parent := spawnCircle(w, math2d.Vec2{X: 10, Y: 20})
	w.SetHeight(parent, scene.Height{Z: 0})
	w.SetShadowStyle(parent, scene.ShadowStyle{Scale: 2})

	w.SpawnAttached(parent, math2d.Vec2{X: 1, Y: -1}, 0, scene.Bundle{
		CircleSprite: &scene.CircleSprite{
			Radius:     0.5,
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(1, 1, 1, 1)},
		},
	})

	commands := w.RenderCommands()
	if n := countEllipses(commands); n != 1 {
		t.Fatalf("composite emitted %d shadows, want 1", n)
	}
	shadow := firstEllipse(t, commands)
	if shadow.RadiusX != 4 {
		t.Errorf("composite shadow radius = %v, want the parent's styled 4", shadow.RadiusX)
	}
}

// A ground sprite is a decal in the ground plane and casts no shadow of its
// own: ShadowStyle on it is a documented no-op.
func TestShadowStyleGroundPlaneIsNoOp(t *testing.T) {
	w := scene.NewWorld()
	w.SetProjection(shadowProjection())
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 5, Y: 5}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetCircleSprite(id, scene.CircleSprite{
		Radius:     2,
		BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(1, 1, 1, 1), Plane: scene.PlaneGround},
	})
	w.SetHeight(id, scene.Height{Z: 0})
	w.SetShadowStyle(id, scene.ShadowStyle{})

	if n := countEllipses(w.RenderCommands()); n != 0 {
		t.Errorf("ground-plane sprite emitted %d contact shadows, want 0", n)
	}
}

// ShadowStyle is presentation-only: two worlds that differ only in it hash
// identically on every checksum.
func TestShadowStyleNeverEntersChecksums(t *testing.T) {
	build := func(withStyle bool) *scene.World {
		w := scene.NewWorld()
		id := spawnCircle(w, math2d.Vec2{X: 1, Y: 1})
		w.SetHeight(id, scene.Height{Z: 0})
		if withStyle {
			w.SetShadowStyle(id, scene.ShadowStyle{Scale: 3, Alpha: 0.2, Disable: true})
		}
		return w
	}

	without, with := build(false), build(true)
	if without.Checksum() != with.Checksum() {
		t.Error("ShadowStyle changed Checksum")
	}
	if without.SimChecksum() != with.SimChecksum() {
		t.Error("ShadowStyle changed SimChecksum")
	}
	if without.ContentChecksum() != with.ContentChecksum() {
		t.Error("ShadowStyle changed ContentChecksum")
	}
}

// ShadowFootprint is authored visual content: it moves the content
// fingerprint and only that.
func TestShadowFootprintIsContentNotSim(t *testing.T) {
	build := func(footprint math2d.Vec2) *scene.World {
		w := scene.NewWorld()
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 1, Y: 1}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetImageSprite(id, scene.ImageSprite{
			Size:            math2d.Vec2{X: 4, Y: 4},
			ShadowFootprint: footprint,
		})
		return w
	}

	plain := build(math2d.Vec2{})
	authored := build(math2d.Vec2{X: 1.5, Y: 0.5})

	if plain.ContentChecksum() == authored.ContentChecksum() {
		t.Error("ShadowFootprint did not move ContentChecksum")
	}
	if plain.SimChecksum() != authored.SimChecksum() {
		t.Error("ShadowFootprint moved SimChecksum")
	}
}

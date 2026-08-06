package raster

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// TestGolden25DGroundAndHeight is the composition proof for 2.5D: the tilted
// camera, the two sprite planes and the derived height presentation have unit
// tests each, and this is the one place they have to agree with one another.
//
// What the image shows, against a camera tilted 60 degrees off vertical:
//
//   - A dark green ground slab, laid flat, foreshortened to half its world
//     height.
//   - Two squares of identical world size sitting on it. The left one is
//     PlaneGround and comes out half as tall as it is wide; the right one is
//     PlaneUpright and stays square, because a standing sprite faces the
//     camera instead of lying in the plane being foreshortened.
//   - Three balls at increasing height above the same ground row. The tilt
//     lifts them up the screen; their shadows stay on the ground, shrink and
//     fade with height, and travel right because the light comes from the
//     left. The leftmost is barely off the ground and its shadow is still
//     visible below it rather than hidden behind the body, which is the
//     minimum shadow offset earning its place.
//
// The shadows are round here, and that is the point of the fixture. A light
// only 20 degrees off vertical throws a mark barely longer than the body and
// barely to the side, so the height reads almost entirely from the lift. An
// earlier model leaned the shadow with a shear and drew a long thin scratch
// instead, which looked like more of a cue and was in fact the shadow losing
// exactly as much width as it gained length.
func TestGolden25DGroundAndHeight(t *testing.T) {
	cam := render.Camera{
		World:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 120, Y: 80}),
		Viewport:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 240, Y: 160}),
		DepthScale: 0.5,
	}

	world := scene.NewWorld()
	world.SetProjection(render.GroundProjection(cam, render.ProjectionLight{
		Direction: math2d.Vec2{X: -1},
		Tilt:      0.22,
		Softness:  0.6,
	}))

	rect := func(area math2d.Rect, layer int, plane scene.SpritePlane, color render.Color) {
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{Position: area.Center(), Scale: math2d.Vec2{X: 1, Y: 1}})
		half := area.Size().Mul(0.5)
		world.SetRectSprite(id, scene.RectSprite{
			Rect:       math2d.NewRectFromCorners(half.Mul(-1), half),
			BaseSprite: scene.BaseSprite{Layer: layer, Color: color, Plane: plane},
		})
	}

	square := func(centre math2d.Vec2, plane scene.SpritePlane, color render.Color) {
		const half = 9
		rect(math2d.NewRectFromCorners(
			centre.Sub(math2d.Vec2{X: half, Y: half}),
			centre.Add(math2d.Vec2{X: half, Y: half}),
		), 1, plane, color)
	}

	rect(math2d.NewRect(math2d.Vec2{X: 6, Y: 26}, math2d.Vec2{X: 108, Y: 40}),
		0, scene.PlaneGround, render.RGBA(0.16, 0.28, 0.20, 1))

	// Same world size, same row: every difference on screen is the plane.
	square(math2d.Vec2{X: 22, Y: 46}, scene.PlaneGround, render.RGBA(0.85, 0.80, 0.45, 0.9))
	square(math2d.Vec2{X: 48, Y: 46}, scene.PlaneUpright, render.RGBA(0.45, 0.65, 0.88, 0.9))

	// Same ground row, same radius: every difference on screen is the height.
	for i, z := range []float64{0.1, 4, 9} {
		ball := world.Spawn()
		world.SetTransform(ball, scene.Transform{
			Position: math2d.Vec2{X: 72 + float64(i)*22, Y: 46},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetCircleSprite(ball, scene.CircleSprite{
			Radius:     6,
			BaseSprite: scene.BaseSprite{Layer: 2, Color: render.RGBA(0.90, 0.45, 0.35, 1)},
		})
		world.SetHeight(ball, scene.Height{Z: z})
	}

	renderer := &Renderer{
		Width:  240,
		Height: 160,
		Camera: cam,
		Clear:  goldenClear,
	}

	assertGolden(t, "projection_25d", renderer.Render(world.RenderCommands()))
}

// TestGolden25DContactShadow is the composition proof for the contact shadow
// and its per-entity style. Four balls of identical world size against the
// same tilted, lit camera as the fixture above:
//
//   - The first stands on the ground with no ShadowStyle: no shadow, the 2D
//     behaviour every scene had before the component existed.
//   - The second adds a zero ShadowStyle: the contact shadow appears, shaped
//     by the projection's own light.
//   - The third styles it -- wider, fainter, nudged toward the viewer.
//   - The fourth is airborne with Disable: the body lifts, no shadow at all.
func TestGolden25DContactShadow(t *testing.T) {
	cam := render.Camera{
		World:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 120, Y: 80}),
		Viewport:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 240, Y: 160}),
		DepthScale: 0.5,
	}

	world := scene.NewWorld()
	world.SetProjection(render.GroundProjection(cam, render.ProjectionLight{
		Direction: math2d.Vec2{X: -1},
		Tilt:      0.22,
		Softness:  0.6,
	}))

	ground := world.Spawn()
	world.SetTransform(ground, scene.Transform{
		Position: math2d.Vec2{X: 60, Y: 46},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetRectSprite(ground, scene.RectSprite{
		Rect:       math2d.NewRectFromCorners(math2d.Vec2{X: -54, Y: -20}, math2d.Vec2{X: 54, Y: 20}),
		BaseSprite: scene.BaseSprite{Layer: 0, Color: render.RGBA(0.16, 0.28, 0.20, 1), Plane: scene.PlaneGround},
	})

	styles := []struct {
		z     float64
		style *scene.ShadowStyle
	}{
		{z: 0, style: nil},
		{z: 0, style: &scene.ShadowStyle{}},
		{z: 0, style: &scene.ShadowStyle{Scale: 1.6, Alpha: 0.5, Offset: math2d.Vec2{Y: 2}}},
		{z: 6, style: &scene.ShadowStyle{Disable: true}},
	}
	for i, s := range styles {
		ball := world.Spawn()
		world.SetTransform(ball, scene.Transform{
			Position: math2d.Vec2{X: 24 + float64(i)*24, Y: 46},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetCircleSprite(ball, scene.CircleSprite{
			Radius:     6,
			BaseSprite: scene.BaseSprite{Layer: 2, Color: render.RGBA(0.90, 0.45, 0.35, 1)},
		})
		world.SetHeight(ball, scene.Height{Z: s.z})
		if s.style != nil {
			world.SetShadowStyle(ball, *s.style)
		}
	}

	renderer := &Renderer{
		Width:  240,
		Height: 160,
		Camera: cam,
		Clear:  goldenClear,
	}

	assertGolden(t, "projection_25d_contact_shadow", renderer.Render(world.RenderCommands()))
}

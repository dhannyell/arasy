package raster

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// TestGoldenIsometricView is the composition proof for Camera.View: the
// projection, the two sprite planes, the depth sort's axis and the height
// presentation all have unit tests each, and this is the one place they have
// to agree with one another and with the rasteriser.
//
// The world here is an eight-by-eight square grid, and everything below is
// written in it. Nothing corrects for the projection.
//
// What the image shows:
//
//   - A checkerboard of unit squares, which the view turns into diamonds
//     because they are PlaneGround and lie in the plane being projected.
//   - A row of posts down the middle, which stay upright rectangles because
//     they are PlaneUpright. Their spacing follows the grid diagonally while
//     their shape does not follow it at all -- that is the whole difference
//     between the two planes.
//   - Three balls at increasing height over the same tile. They rise straight
//     up the screen rather than along a grid line, which is LiftAxis; their
//     shadows stay on the tile and shrink.
//   - A raised slab at the back, which is the one thing here that is both a
//     rectangle and off the ground.
func TestGoldenIsometricView(t *testing.T) {
	const tiles = 8

	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: -170, Y: -58}, math2d.Vec2{X: 340, Y: 200}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 340, Y: 200}),
		View:     math2d.Affine{A: 20, C: -20, B: 10, D: 10},
	}

	world := scene.NewWorld()
	world.SetProjection(render.GroundProjection(cam, render.ProjectionLight{Softness: 0.6}))

	// On this grid the direction away from the viewer is the diagonal.
	world.SetSortAxis(math2d.Vec2{X: 1, Y: 1})

	rect := func(area math2d.Rect, layer int, plane scene.SpritePlane, colour render.Color) scene.EntityID {
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{Position: area.Center(), Scale: math2d.Vec2{X: 1, Y: 1}})
		half := area.Size().Mul(0.5)
		world.SetRectSprite(id, scene.RectSprite{
			Rect:       math2d.NewRectFromCorners(half.Mul(-1), half),
			BaseSprite: scene.BaseSprite{Layer: layer, Color: colour, Plane: plane},
		})

		return id
	}

	for y := range tiles {
		for x := range tiles {
			colour := render.RGBA(0.35, 0.60, 0.52, 1)
			if (x+y)%2 == 1 {
				colour = render.RGBA(0.26, 0.48, 0.42, 1)
			}

			rect(math2d.NewRect(
				math2d.Vec2{X: float64(x) - 0.5, Y: float64(y) - 0.5},
				math2d.Vec2{X: 1, Y: 1},
			), 0, scene.PlaneGround, colour)
		}
	}

	// Upright, and standing on their own bottom edge so the sort reads the
	// base off the rect without an offset.
	for _, feet := range []math2d.Vec2{{X: 1, Y: 6}, {X: 3, Y: 4}, {X: 5, Y: 2}} {
		rect(math2d.NewRectFromCorners(
			math2d.Vec2{X: feet.X - 0.15, Y: feet.Y - 1.4},
			math2d.Vec2{X: feet.X + 0.15, Y: feet.Y},
		), 1, scene.PlaneUpright, render.RGBA(0.93, 0.91, 0.95, 1))
	}

	// Same tile, same radius: every difference on screen is the height.
	for i, z := range []float64{0.05, 1.6, 3.2} {
		ball := world.Spawn()
		world.SetTransform(ball, scene.Transform{
			Position: math2d.Vec2{X: 6, Y: 1 + float64(i)*2.5},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetCircleSprite(ball, scene.CircleSprite{
			Radius: 0.4,
			BaseSprite: scene.BaseSprite{
				Layer:      1,
				Color:      render.RGBA(0.95, 0.46, 0.38, 1),
				SortOffset: math2d.Vec2{Y: -0.4},
			},
		})
		world.SetHeight(ball, scene.Height{Z: z})
	}

	slab := rect(math2d.NewRect(
		math2d.Vec2{X: -2.2, Y: -2.2}, math2d.Vec2{X: 2.6, Y: 2.6},
	), 1, scene.PlaneGround, render.RGBA(0.55, 0.78, 0.70, 1))
	world.SetHeight(slab, scene.Height{Z: 1})

	renderer := &Renderer{
		Width:  340,
		Height: 200,
		Camera: cam,
		Clear:  goldenClear,
	}

	assertGolden(t, "isometric_view", renderer.Render(world.RenderCommands()))
}

package raster

import (
	"image"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestGoldenDebugOverlay(t *testing.T) {
	world := goldenDebugWorld()
	const expectedChecksum uint64 = 3452738200581198465
	if got := world.Checksum(); got != expectedChecksum {
		t.Fatalf("debug world checksum = %d, want %d", got, expectedChecksum)
	}

	commands := world.RenderCommands()
	commands = append(commands, world.DebugCommands(
		scene.DebugGrid|scene.DebugActiveGrid|scene.DebugCandidateGrid|scene.DebugColliders|
			scene.DebugContacts|scene.DebugVelocities|scene.DebugEntityIDs,
		scene.WithDebugBounds(goldenDebugBounds()),
		scene.WithDebugCellSize(8),
	)...)

	assertFontZeroGolden(t, "debug_overlay", goldenRenderer().Render(commands))
}

func TestGoldenCameraAspect(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Fill: render.RGBA(0.16, 0.19, 0.22, 1),
		},
		render.DrawCircle{
			Center:      math2d.Vec2{X: 50, Y: 37.5},
			Radius:      22,
			Fill:        render.RGBA(0.25, 0.72, 0.5, 0.8),
			Stroke:      render.RGBA(1, 0.95, 0.55, 1),
			StrokeWidth: 1.2,
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 6, Y: 6}, math2d.Vec2{X: 88, Y: 63}),
			Stroke:      render.RGBA(0.95, 0.95, 1, 1),
			StrokeWidth: 1,
		},
	}

	img := image.NewRGBA(image.Rect(0, 0, 260, 150))
	leftRenderer := Renderer{
		Width:  260,
		Height: 150,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 125, Y: 150}),
			Aspect:   render.AspectFit,
		},
		Clear: goldenClear,
	}
	left := leftRenderer.Render(commands)
	rightRenderer := Renderer{
		Width:  260,
		Height: 150,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{X: 135, Y: 0}, math2d.Vec2{X: 125, Y: 150}),
			Aspect:   render.AspectExpand,
		},
		Clear: render.RGBA(0, 0, 0, 0),
	}
	right := rightRenderer.Render(commands)

	copy(img.Pix, left.Pix)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 135; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, right.RGBAAt(x, y))
		}
	}

	assertGolden(t, "camera_aspect", img)
}

func goldenDebugBounds() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75})
}

func goldenDebugWorld() *scene.World {
	world := scene.NewWorld()

	spawnGoldenCircle(world, math2d.Vec2{X: 26, Y: 32}, math2d.Vec2{X: 8, Y: -4}, 9, render.RGBA(0.2, 0.65, 1, 0.82), 1)
	spawnGoldenCircle(world, math2d.Vec2{X: 41, Y: 33}, math2d.Vec2{X: -6, Y: 5}, 8, render.RGBA(1, 0.45, 0.28, 0.82), 2)
	spawnGoldenCircle(world, math2d.Vec2{X: 74, Y: 48}, math2d.Vec2{X: -3, Y: -8}, 6, render.RGBA(0.62, 1, 0.42, 0.82), 3)
	spawnGoldenRect(world, math2d.Vec2{X: 57, Y: 29}, math2d.Vec2{X: 9, Y: 7}, math2d.Vec2{X: 2, Y: 4}, render.RGBA(0.92, 0.76, 0.22, 0.78), 0)
	spawnGoldenRect(world, math2d.Vec2{X: 66, Y: 35}, math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: -2, Y: -3}, render.RGBA(0.76, 0.46, 1, 0.72), 0)

	world.Step(1.0/60.0, scene.StepConfig{
		Bounds: goldenDebugBounds(),
		Physics: scene.PhysicsConfig{
			CellSize: 8,
		},
	})

	return world
}

func spawnGoldenCircle(world *scene.World, position, velocity math2d.Vec2, radius float64, color render.Color, layer int) {
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetVelocity(entity, scene.Velocity{Linear: velocity})
	world.SetBody(entity, scene.Body{InvMass: 1, Restitution: 0.65})
	world.SetCircleCollider(entity, scene.CircleCollider{Radius: radius})
	world.SetCircleSprite(entity, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: layer, Color: color},
		Radius:     radius,
	})
}

func spawnGoldenRect(world *scene.World, position, halfExtents, velocity math2d.Vec2, color render.Color, layer int) {
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetVelocity(entity, scene.Velocity{Linear: velocity})
	world.SetBody(entity, scene.Body{InvMass: 1, Restitution: 0.4})
	world.SetRectCollider(entity, scene.RectCollider{HalfExtends: halfExtents})
	world.SetRectSprite(entity, scene.RectSprite{
		BaseSprite:  scene.BaseSprite{Layer: layer, Color: color},
		Rect:        math2d.NewRectFromCorners(position.Sub(halfExtents), position.Add(halfExtents)),
		Stroke:      render.RGBA(1, 1, 1, 0.58),
		StrokeWidth: 0.8,
	})
}

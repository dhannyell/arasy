package scene_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// tiltedCamera maps a square world onto a square viewport, so the only thing
// that can stretch a shape on screen is the tilt.
func tiltedCamera(depthScale float64) render.Camera {
	return render.Camera{
		World:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		DepthScale: depthScale,
	}
}

func tiltedWorld(depthScale float64) *scene.World {
	world := scene.NewWorld()
	world.SetProjection(render.Projection{DepthScale: depthScale})
	return world
}

// screenOffset follows a rect or image corner the way the backends do: through
// the command's own transform, then through the camera.
func screenOffset(t *testing.T, cmd render.Command, cam render.Camera, centre, corner math2d.Vec2) math2d.Vec2 {
	t.Helper()

	affine := commandTransform(t, cmd)

	return cam.WorldToScreen(affine.Apply(corner)).Sub(cam.WorldToScreen(affine.Apply(centre)))
}

// circleScreenHalfHeight follows a circle command the way the backends do:
// through the camera's radius conversion when it carries no transform, and
// boundary point by boundary point when it does.
func circleScreenHalfHeight(cmd render.DrawCircle, cam render.Camera) float64 {
	if cmd.Transform.IsIdentity() {
		return cam.WorldRadiiToScreen(math2d.Vec2{X: cmd.Radius, Y: cmd.Radius}).Y
	}

	top := cmd.Center.Add(math2d.Vec2{Y: cmd.Radius})

	return cam.WorldToScreen(cmd.Transform.Apply(top)).Y -
		cam.WorldToScreen(cmd.Transform.Apply(cmd.Center)).Y
}

func commandTransform(t *testing.T, cmd render.Command) math2d.Affine {
	t.Helper()

	switch c := cmd.(type) {
	case render.DrawCircle:
		return c.Transform
	case render.DrawRect:
		return c.Transform
	case render.DrawImage:
		return c.Transform
	default:
		t.Fatalf("unexpected command %T", cmd)
		return math2d.Affine{}
	}
}

func spawnRect(w *scene.World, centre math2d.Vec2, half float64, plane scene.SpritePlane) {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: centre, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetRectSprite(id, scene.RectSprite{
		Rect: math2d.NewRectFromCorners(
			centre.Sub(math2d.Vec2{X: half, Y: half}),
			centre.Add(math2d.Vec2{X: half, Y: half}),
		),
		BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1), Plane: plane},
	})
}

func TestSpritePlaneDecidesWhetherTheCameraTiltSquashesTheSprite(t *testing.T) {
	const half = 10

	tests := []struct {
		name           string
		plane          scene.SpritePlane
		wantHalfHeight float64
	}{
		{"upright sprite keeps its shape", scene.PlaneUpright, half},
		{"ground sprite foreshortens with the ground", scene.PlaneGround, half / 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cam := tiltedCamera(0.5)
			world := tiltedWorld(cam.DepthScale)

			centre := math2d.Vec2{X: 50, Y: 50}
			spawnRect(world, centre, half, tt.plane)

			cmd := world.RenderCommands()[0]
			top := screenOffset(t, cmd, cam, centre, centre.Add(math2d.Vec2{Y: half}))
			side := screenOffset(t, cmd, cam, centre, centre.Add(math2d.Vec2{X: half}))

			if math.Abs(top.Y-tt.wantHalfHeight) > 1e-9 {
				t.Errorf("vertical half-extent on screen: got %v, want %v", top.Y, tt.wantHalfHeight)
			}
			if math.Abs(side.X-half) > 1e-9 {
				t.Errorf("tilt moved the horizontal axis: got %v, want %v", side.X, half)
			}
		})
	}
}

// A circle's radius is a shape, not a scalar: the camera foreshortens it, so a
// ground circle draws as an ellipse and an upright one has to be stood back up
// like any other sprite.
func TestCircleSpriteFollowsItsPlaneLikeEveryOtherSprite(t *testing.T) {
	cam := tiltedCamera(0.5)
	const radius = 10

	for plane, wantHalfHeight := range map[scene.SpritePlane]float64{
		scene.PlaneUpright: radius,
		scene.PlaneGround:  radius / 2,
	} {
		world := tiltedWorld(cam.DepthScale)

		centre := math2d.Vec2{X: 50, Y: 50}
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{Position: centre, Scale: math2d.Vec2{X: 1, Y: 1}})
		world.SetCircleSprite(id, scene.CircleSprite{
			Radius:     radius,
			BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1), Plane: plane},
		})

		got := circleScreenHalfHeight(world.RenderCommands()[0].(render.DrawCircle), cam)

		if math.Abs(got-wantHalfHeight) > 1e-9 {
			t.Errorf("plane %v: vertical radius on screen %v, want %v", plane, got, wantHalfHeight)
		}
	}
}

// The sprite's position must foreshorten even when its shape does not: an
// upright sprite that ignored the tilt entirely would drift away from the
// ground it is standing on.
func TestUprightSpriteStillMovesWithTheForeshortenedGround(t *testing.T) {
	cam := tiltedCamera(0.5)
	world := tiltedWorld(cam.DepthScale)

	centre := math2d.Vec2{X: 50, Y: 90}
	spawnRect(world, centre, 10, scene.PlaneUpright)

	cmd := world.RenderCommands()[0]
	got := cam.WorldToScreen(commandTransform(t, cmd).Apply(centre))

	if !vecNear(got, cam.WorldToScreen(centre), 1e-9) {
		t.Errorf("sprite centre on screen: got %v, want %v", got, cam.WorldToScreen(centre))
	}
}

// Order matters and is invisible in the common case, so it gets its own test:
// a quarter-turn decorative rotation has to happen in the sprite's own upright
// frame. Applied the other way round, the tilt would leak into the rotation and
// the sprite would lose its proportions as it spins.
func TestPlaneCorrectionWrapsTheDecorativeTransform(t *testing.T) {
	cam := tiltedCamera(0.5)
	world := tiltedWorld(cam.DepthScale)

	const arm = 10
	centre := math2d.Vec2{X: 50, Y: 50}
	spawnRect(world, centre, arm, scene.PlaneUpright)
	world.SetSpriteTransform(world.Entities()[0], scene.SpriteTransform{M: math2d.Rotate(math.Pi / 2)})

	got := screenOffset(t, world.RenderCommands()[0], cam, centre, centre.Add(math2d.Vec2{X: arm}))

	// A quarter turn sends the horizontal arm straight up, at its full length.
	// Composing in the wrong order would halve it, to arm * DepthScale.
	if math.Abs(got.Y-arm) > 1e-9 || math.Abs(got.X) > 1e-9 {
		t.Errorf("rotated arm on screen: got %v, want %v", got, math2d.Vec2{Y: arm})
	}
}

// Touching down is the moment a height cue is most likely to look broken: the
// shadow is at its largest and closest to the body. It must grow smoothly into
// the landing and then simply stop, never flip to the far side of the body or
// blink out one frame early.
func TestShadowHoldsItsGroundAsHeightFallsToZero(t *testing.T) {
	cam := tiltedCamera(0.5)

	world := scene.NewWorld()
	world.SetProjection(render.GroundProjection(cam, render.ProjectionLight{
		Direction: math2d.Vec2{X: -1},
		Tilt:      0.3,
		Softness:  0.6,
	}))

	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 50, Y: 50}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleSprite(id, scene.CircleSprite{
		Radius:     8,
		BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1)},
	})

	previousRadius := 0.0

	for _, z := range []float64{2, 1, 0.5, 0.1, 0.01} {
		world.SetHeight(id, scene.Height{Z: z})

		shadow, ok := world.RenderCommands()[0].(render.DrawEllipse)
		if !ok {
			t.Fatalf("z=%v: no shadow drawn before the body", z)
		}
		if shadow.Fill.A <= 0 || shadow.RadiusX <= 0 || shadow.RadiusY <= 0 {
			t.Fatalf("z=%v: shadow present but invisible: %+v", z, shadow)
		}
		if shadow.Center.Y <= 50 {
			t.Errorf("z=%v: shadow centre %v crossed to the far side of the body", z, shadow.Center)
		}
		if shadow.RadiusX < previousRadius {
			t.Errorf("z=%v: shadow shrank while falling, %v after %v", z, shadow.RadiusX, previousRadius)
		}
		previousRadius = shadow.RadiusX
	}

	world.SetHeight(id, scene.Height{Z: 0})
	if _, isShadow := world.RenderCommands()[0].(render.DrawEllipse); isShadow {
		t.Error("shadow still drawn once the entity is on the ground")
	}
}

func TestSpritePlaneStaysOutOfTheChecksum(t *testing.T) {
	build := func(plane scene.SpritePlane) uint64 {
		world := tiltedWorld(0.5)

		id := world.Spawn()
		world.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 1, Y: 2}, Scale: math2d.Vec2{X: 1, Y: 1}})
		world.SetCircleSprite(id, scene.CircleSprite{
			Radius:     3,
			BaseSprite: scene.BaseSprite{Plane: plane},
		})

		image := world.Spawn()
		world.SetTransform(image, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
		world.SetImageSprite(image, scene.ImageSprite{Size: math2d.Vec2{X: 4, Y: 4}, BaseSprite: scene.BaseSprite{Plane: plane}})

		return world.Checksum()
	}

	if build(scene.PlaneUpright) != build(scene.PlaneGround) {
		t.Error("sprite plane moved the checksum: presentation leaked into simulation")
	}
}

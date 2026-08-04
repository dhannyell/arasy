// Package groundplane is the smallest 2.5D scene the engine can draw: a
// camera tilted away from straight-down, a ball with height bouncing around a
// court painted on the ground, and the shadow the engine derives from those
// two facts.
//
// All of the 2.5D is two lines — Camera.DepthScale in Setup, and handing that
// camera to render.GroundProjection in Init. Everything below them is the same
// plain 2D simulation it would be without any tilt at all: the ball moves,
// bounces off the bounds and settles in unforeshortened world units, because
// foreshortening is a fact about the camera and never about the world.
//
// For a full game built on the same foundation, see the dodgeball example.
package groundplane

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

const (
	// depthScale is the classic 2:1 view of pixel-art dimetric projection: the
	// camera sits 60 degrees off vertical, so the ground keeps half of its
	// depth on screen. Change it to 1 and the whole scene turns top-down —
	// the ball still bounces, it just has nowhere on screen to bounce to.
	depthScale = 0.5

	courtWidth  = 160.0
	courtHeight = 90.0

	ballRadius  = 4.0
	serveHeight = 12.0
)

var (
	courtColor = render.RGBA8(38, 74, 56, 255)
	markColor  = render.RGBA8(126, 178, 148, 190)
	postColor  = render.RGBA8(224, 214, 188, 255)
	ballColor  = render.RGBA8(232, 118, 92, 255)
	clearColor = render.RGBA8(18, 24, 30, 255)
	// The court sits low in the world on purpose: the tilt lifts a ball by
	// tan(60 degrees) of its height, and the headroom above the court is where
	// that lift goes.
	courtBounds = math2d.NewRect(math2d.Vec2{X: 12, Y: 34}, math2d.Vec2{X: 136, Y: 44})
)

type Game struct {
	ball     scene.EntityID
	commands []render.Command
}

func (g *Game) Setup() runtime.Config {
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: courtWidth, Y: courtHeight})

	return runtime.Config{
		Camera: render.Camera{
			World:      world,
			Viewport:   world,
			Aspect:     render.AspectFit,
			DepthScale: depthScale,
		},
		Clear:        clearColor,
		Bounds:       courtBounds,
		SimulationHz: 60,
	}
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	// Deriving the projection from the camera is what keeps the picture
	// honest: change depthScale above and the lift, the shadow's size and its
	// lean all follow, with no second number to keep in sync.
	ctx.World.SetProjection(render.GroundProjection(ctx.Camera(), render.ProjectionLight{
		Direction: math2d.Vec2{X: -1, Y: -1},
		Tilt:      0.18,
		Softness:  0.5,
	}))

	g.spawnCourt(ctx.World)
	g.spawnPosts(ctx.World)
	g.ball = g.spawnBall(ctx.World)
	g.serve(ctx)
}

// spawnCourt paints the pitch. Both sprites are PlaneGround, so the tilt
// foreshortens them: the centre circle is drawn as a circle and lands on
// screen as an ellipse, which is exactly what a circle painted on the ground
// looks like from an angle.
func (g *Game) spawnCourt(world *scene.World) {
	world.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: courtBounds.Center()},
		RectSprite: &scene.RectSprite{
			Rect:       courtBounds,
			BaseSprite: scene.BaseSprite{Layer: 0, Color: courtColor, Plane: scene.PlaneGround},
		},
	})

	world.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: courtBounds.Center()},
		CircleSprite: &scene.CircleSprite{
			Radius:     14,
			BaseSprite: scene.BaseSprite{Layer: 1, Color: markColor, Plane: scene.PlaneGround},
		},
	})
}

// spawnPosts stands two markers at the ends of the court. They are the default
// PlaneUpright, so the same tilt that flattens the court leaves them their
// full height — the difference between something lying on the ground and
// something standing on it.
func (g *Game) spawnPosts(world *scene.World) {
	for _, x := range []float64{courtBounds.Min.X + 6, courtBounds.Max.X - 6} {
		foot := math2d.Vec2{X: x, Y: courtBounds.Center().Y}
		world.SpawnBundle(scene.Bundle{
			Transform: &scene.Transform{Position: foot},
			RectSprite: &scene.RectSprite{
				Rect: math2d.NewRectFromCorners(
					foot.Sub(math2d.Vec2{X: 2, Y: 14}),
					foot.Add(math2d.Vec2{X: 2, Y: 2}),
				),
				BaseSprite: scene.BaseSprite{Layer: 2, Color: postColor},
			},
		})
	}
}

// spawnBall is the only entity that needs the 2.5D components: Height carries
// the altitude, Ballistics falls and bounces it. Both are simulation state,
// hashed and replayed like any other -- it is only how they turn into pixels
// that the projection decides.
func (g *Game) spawnBall(world *scene.World) scene.EntityID {
	return world.SpawnBundle(scene.Bundle{
		CircleSprite: &scene.CircleSprite{
			Radius:     ballRadius,
			BaseSprite: scene.BaseSprite{Layer: 3, Color: ballColor},
		},
		Body: &scene.Body{InvMass: 1, Restitution: 1},
		Ballistics: &scene.Ballistics{
			Gravity:        90,
			BounceDamping:  0.62,
			BounceFriction: 0.98,
			RollFriction:   6,
			RestVZ:         2,
		},
	})
}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	// Every bounce costs the ball some energy, so eventually it settles and
	// rolls to a stop. Serving again keeps the height cue on screen; drawing
	// the serve from ctx.Rand keeps the demo replayable.
	if height, ok := ctx.World.Height(g.ball); ok && height.Z <= 0 && height.VZ == 0 {
		if velocity, ok := ctx.World.Velocity(g.ball); ok && velocity.Linear.LenSq() == 0 {
			g.serve(ctx)
		}
	}
}

func (g *Game) serve(ctx *runtime.UpdateContext) {
	position := math2d.Vec2{
		X: courtBounds.Min.X + ctx.Rand.Float64()*courtBounds.Width(),
		Y: courtBounds.Min.Y + ctx.Rand.Float64()*courtBounds.Height(),
	}

	ctx.World.SetTransform(g.ball, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(g.ball, scene.Velocity{Linear: math2d.Vec2{
		X: 24 + ctx.Rand.Float64()*36,
		Y: 16 + ctx.Rand.Float64()*24,
	}})
	ctx.World.SetHeight(g.ball, scene.Height{Z: serveHeight})
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	g.commands = ctx.World.RenderCommandsInto(g.commands[:0], ctx.Alpha)
	return g.commands
}

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
// The third thing to watch is the one nobody writes a line for: the ball and
// the posts share a layer, so which is drawn in front is decided by how deep
// into the court each one stands. Watch the near post and the ball goes behind
// it and comes back out again.
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

	// The pitch and its marking are strata: one is painted over the other and
	// no depth relation between them would mean anything, so they get a layer
	// each. Everything that stands on the pitch shares one layer instead, and
	// lets the depth sort decide -- which is the whole difference between the
	// two ideas.
	layerPitch    = 0
	layerMarking  = 1
	layerStanding = 2
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
			Rect: math2d.NewRectFromCorners(
				courtBounds.Size().Mul(-0.5),
				courtBounds.Size().Mul(0.5),
			),
			BaseSprite: scene.BaseSprite{Layer: layerPitch, Color: courtColor, Plane: scene.PlaneGround},
		},
	})

	world.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: courtBounds.Center()},
		CircleSprite: &scene.CircleSprite{
			Radius:     14,
			BaseSprite: scene.BaseSprite{Layer: layerMarking, Color: markColor, Plane: scene.PlaneGround},
		},
	})
}

// spawnPosts stands two markers at the ends of the court. They are the default
// PlaneUpright, so the same tilt that flattens the court leaves them their
// full height — the difference between something lying on the ground and
// something standing on it.
//
// They share layerStanding with the ball, so which of them is drawn in front
// is decided every frame by how deep into the court each one stands. A post is
// four times taller than the ball is wide, and sorting by their centres would
// put the post behind a ball that is level with it; sorting by their bases,
// which is the default, gets it right. The posts carry no collider on purpose:
// they are there to be sorted against, not bounced off.
//
// One stands near the back of the court and the other near the front, both set
// in from the ends where the ball actually spends its time. The near post is
// the one to watch: the ball passes behind it and comes back out in front as
// it crosses, and nothing in this file says when. Against the far post the
// ball is almost always in front, because almost all of the court is nearer
// than that — which is the same rule giving a different answer, not a
// different rule.
func (g *Game) spawnPosts(world *scene.World) {
	for _, foot := range []math2d.Vec2{
		{
			X: courtBounds.Min.X + courtBounds.Width()*0.3,
			Y: courtBounds.Min.Y + courtBounds.Height()*0.28,
		},
		{
			X: courtBounds.Min.X + courtBounds.Width()*0.7,
			Y: courtBounds.Min.Y + courtBounds.Height()*0.72,
		},
	} {
		world.SpawnBundle(scene.Bundle{
			Transform: &scene.Transform{Position: foot},
			RectSprite: &scene.RectSprite{
				Rect: math2d.NewRectFromCorners(
					math2d.Vec2{X: -2, Y: -14},
					math2d.Vec2{X: 2, Y: 2},
				),
				BaseSprite: scene.BaseSprite{Layer: layerStanding, Color: postColor},
			},
		})
	}
}

// spawnBall is the only entity that needs the 2.5D components: Height carries
// the altitude, Ballistics falls and bounces it. Both are simulation state,
// hashed and replayed like any other -- it is only how they turn into pixels
// that the projection decides.
//
// The collider is what keeps the ball on the court. Config.Bounds only acts on
// entities that have one, so without it the ball rolls straight out of the
// world -- the altitude bounce is Ballistics and needs nothing, but the four
// walls are ordinary 2D collision and do.
//
// Height deliberately stays out of the depth key, and this is the entity that
// shows why: a ball flying over a post passes behind it and comes out the
// other side, instead of snapping in front the moment it leaves the ground.
func (g *Game) spawnBall(world *scene.World) scene.EntityID {
	return world.SpawnBundle(scene.Bundle{
		CircleSprite: &scene.CircleSprite{
			Radius:     ballRadius,
			BaseSprite: scene.BaseSprite{Layer: layerStanding, Color: ballColor},
		},
		CircleCollider: &scene.CircleCollider{Radius: ballRadius},
		Body:           &scene.Body{InvMass: 1, Restitution: 1},
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

	// The serve goes in any direction, not just down and to the right. A ball
	// that only ever travels one way meets the posts from one side and the
	// depth order never gets to change its mind about anything.
	velocity := math2d.Vec2{
		X: 24 + ctx.Rand.Float64()*36,
		Y: 16 + ctx.Rand.Float64()*24,
	}
	if ctx.Rand.Float64() < 0.5 {
		velocity.X = -velocity.X
	}
	if ctx.Rand.Float64() < 0.5 {
		velocity.Y = -velocity.Y
	}

	ctx.World.SetTransform(g.ball, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(g.ball, scene.Velocity{Linear: velocity})
	ctx.World.SetHeight(g.ball, scene.Height{Z: serveHeight})
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	g.commands = ctx.World.RenderCommandsInto(g.commands[:0], ctx.Alpha)
	return g.commands
}

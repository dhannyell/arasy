package outpost

import (
	"math"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

const (
	viewportWidth  = 1600
	viewportHeight = 960

	simulationDt = 1.0 / 60.0

	unitCount  = 2
	unitRadius = 2.1
)

// isoView is the entire projection: screenX = x - y, screenY = (x + y) / 2,
// the classic 2:1 dimetric view. The simulation below never mentions it
// again -- tiles stay squares, clicks come back through ScreenToWorld.
var isoView = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}

var unitBodies = [unitCount]render.Color{
	render.RGBA8(74, 181, 173, 255),
	render.RGBA8(230, 152, 74, 255),
}

var unitStarts = [unitCount][2]int{{2, 5}, {6, 2}}

type UnitState struct {
	TileI, TileJ int

	// A hop in flight: from and to tiles, and progress 0..1.
	Hopping      bool
	FromI, FromJ int
	ToI, ToJ     int
	T            float64
}

type State struct {
	// Selected is the index of the chosen unit, -1 for none. It is game
	// state, not decoration: a click means something different because of it,
	// so a replay has to carry it.
	Selected int
	Units    [unitCount]UnitState
}

type Game struct {
	runtime.StateSlot[State]

	units [unitCount]scene.EntityID

	// Presentation-only, rebuilt or refreshed without entering State.
	static     []render.Command
	projection render.Projection
	hoverI     int
	hoverJ     int
	hovered    bool

	commands []render.Command
}

func NewGame() *Game {
	return &Game{}
}

func (g *Game) Setup() runtime.Config {
	// Camera.World frames *view* space: the board's diamond after isoView,
	// plus headroom for the peaks above and the cliffs below.
	return runtime.Config{
		Camera: render.Camera{
			World: math2d.NewRect(
				math2d.Vec2{X: -72, Y: -14},
				math2d.Vec2{X: 160, Y: 96},
			),
			Viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: viewportWidth, Y: viewportHeight},
			),
			Aspect: render.AspectFit,
			View:   isoView,
		},
		Clear:                render.RGBA8(24, 26, 40, 255),
		SimulationHz:         60,
		FPS:                  60,
		HistoryLen:           600,
		DisableDefaultSplash: true,
	}
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	projection := render.GroundProjection(ctx.Camera(), render.ProjectionLight{
		Direction: math2d.Vec2{X: -1, Y: -0.4},
		Tilt:      0.3,
		Softness:  0.5,
	})
	ctx.World.SetProjection(projection)

	// On this projection screen depth grows along +X+Y, and saying so is what
	// lets two sprites on one layer sort by how deep into the island they
	// stand rather than by plain screen Y.
	ctx.World.SetSortAxis(math2d.Vec2{X: 1, Y: 1})

	// The projection is the whole 2.5D vocabulary in one value: every piece
	// of fake volume below -- cliffs, mountains, buildings -- extrudes along
	// its LiftVector, the same vector that lifts entities with Height, so
	// authored geometry and engine-lifted bodies can never disagree about
	// which way up is.
	g.projection = projection

	g.static = buildStaticBoard(projection)
	spawnProps(ctx.World, projection)

	g.State = State{Selected: -1}
	for u := range g.units {
		i, j := unitStarts[u][0], unitStarts[u][1]
		g.State.Units[u] = UnitState{TileI: i, TileJ: j}
		g.units[u] = ctx.World.SpawnBundle(scene.Bundle{
			Transform: &scene.Transform{Position: tileCenter(i, j)},
			CircleSprite: &scene.CircleSprite{
				Radius:     unitRadius,
				BaseSprite: scene.BaseSprite{Layer: layerActors, Color: unitBodies[u]},
			},
			Height: &scene.Height{},
		})

		// The chassis plate and the highlight ride the body as attachments:
		// they take the unit's position and height without being told, sort
		// as part of it, and the plate -- an upright sprite -- gets the
		// projection's own stand-up correction from the engine.
		ctx.World.SpawnAttached(g.units[u], math2d.Vec2{}, 0, scene.Bundle{
			RectSprite: &scene.RectSprite{
				Rect: math2d.NewRectFromCorners(
					math2d.Vec2{X: -2.5, Y: 0.9},
					math2d.Vec2{X: 2.5, Y: 1.9},
				),
				BaseSprite: scene.BaseSprite{Layer: layerActors, Color: darken(unitBodies[u])},
			},
		})
		ctx.World.SpawnAttached(g.units[u], math2d.Vec2{X: -0.7, Y: -0.7}, 0, scene.Bundle{
			CircleSprite: &scene.CircleSprite{
				Radius:     0.6,
				BaseSprite: scene.BaseSprite{Layer: layerActors, Color: render.RGBA(1, 1, 1, 0.35)},
			},
		})
	}
}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	g.trackHover(ctx)
	g.handleClicks(ctx)

	for u := range g.State.Units {
		g.stepUnit(ctx, u)
	}
}

func (g *Game) trackHover(ctx *runtime.UpdateContext) {
	world := ctx.Camera().ScreenToWorld(ctx.Pointer.Pos)
	g.hoverI, g.hoverJ, g.hovered = tileAt(world)
	if !ctx.Pointer.Inside {
		g.hovered = false
	}
}

func (g *Game) handleClicks(ctx *runtime.UpdateContext) {
	for _, event := range ctx.Pointer.Events {
		if event.Kind != input.EventButtonDown || event.Button != input.PointerPrimary {
			continue
		}
		i, j, ok := tileAt(ctx.Camera().ScreenToWorld(event.Pos))
		if !ok {
			continue
		}
		g.clickTile(i, j)
	}
}

func (g *Game) clickTile(i, j int) {
	if u, ok := g.unitAt(i, j); ok {
		g.State.Selected = u
		return
	}

	selected := g.State.Selected
	if selected < 0 || g.State.Units[selected].Hopping {
		return
	}
	if terrainAt(i, j) != terrainGrass {
		return
	}

	unit := &g.State.Units[selected]
	unit.Hopping = true
	unit.FromI, unit.FromJ = unit.TileI, unit.TileJ
	unit.ToI, unit.ToJ = i, j
	unit.T = 0
	unit.TileI, unit.TileJ = i, j // the destination is claimed on takeoff
}

func (g *Game) unitAt(i, j int) (int, bool) {
	for u, unit := range g.State.Units {
		if unit.TileI == i && unit.TileJ == j {
			return u, true
		}
	}
	return 0, false
}

// hopFlight is the arc for one hop: duration stretches and the apex rises
// with distance, so a short shuffle stays a shuffle and a cross-board jump
// looks like the effort it is.
func hopFlight(from, to math2d.Vec2) (duration, apex float64) {
	distance := to.Sub(from).Len()
	return 0.45 + distance*0.008, 3.2 + distance*0.14
}

func (g *Game) stepUnit(ctx *runtime.UpdateContext, u int) {
	unit := &g.State.Units[u]
	id := g.units[u]

	position := tileCenter(unit.TileI, unit.TileJ)
	z := 0.0

	if unit.Hopping {
		from := tileCenter(unit.FromI, unit.FromJ)
		to := tileCenter(unit.ToI, unit.ToJ)
		duration, apex := hopFlight(from, to)

		unit.T += simulationDt / duration
		if unit.T >= 1 {
			unit.Hopping = false
			unit.T = 1
		} else {
			position = from.Add(to.Sub(from).Mul(unit.T))
			z = apex * 4 * unit.T * (1 - unit.T)
		}
	}

	ctx.World.SetTransform(id, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetHeight(id, scene.Height{Z: z})
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	cmds := append(g.commands[:0], g.static...)
	cmds = g.appendClouds(cmds, ctx.Time)
	cmds = g.appendWater(cmds, ctx.Time)
	cmds = g.appendHover(cmds)
	cmds = g.appendUnitGround(cmds, ctx)

	cmds = ctx.World.RenderCommandsInto(cmds, ctx.Alpha)

	g.commands = cmds
	return cmds
}

// appendClouds drifts soft shadow blobs across the island interior. They are
// ground-plane ellipses, so the same projection that turns tiles into
// diamonds stretches them along the ground -- free coherence.
func (g *Game) appendClouds(cmds []render.Command, time float64) []render.Command {
	cloudFill := render.RGBA(0.04, 0.05, 0.1, 0.10)
	for c := range 3 {
		phase := float64(c) * 2.1
		center := math2d.Vec2{
			X: 40 + 22*math.Sin(time*0.05+phase),
			Y: 30 + 14*math.Cos(time*0.037+phase*1.7),
		}
		cmds = append(cmds, render.DrawEllipse{
			Center:      center,
			RadiusX:     9,
			RadiusY:     7,
			Fill:        cloudFill,
			BaseCommand: render.BaseCommand{LayerValue: layerCloud},
		})
	}
	return cmds
}

func (g *Game) appendWater(cmds []render.Command, time float64) []render.Command {
	for j := 0; j < boardRows; j++ {
		for i := 0; i < boardCols; i++ {
			if terrainAt(i, j) != terrainWater {
				continue
			}
			center := g.projection.Lift(tileCenter(i, j), -waterDepth)
			phase := float64(i*3+j*7) * 1.3
			alpha := 0.10 + 0.08*math.Sin(time*1.1+phase)
			cmds = append(cmds, render.DrawRect{
				Rect: math2d.NewRect(
					center.Add(math2d.Vec2{X: -1.6 + math.Sin(phase)*1.2, Y: -0.4 + math.Cos(phase)*1.4}),
					math2d.Vec2{X: 2.2, Y: 0.7},
				),
				Fill:        render.RGBA(1, 1, 1, alpha),
				BaseCommand: render.BaseCommand{LayerValue: layerGround},
			})
		}
	}
	return cmds
}

func (g *Game) appendHover(cmds []render.Command) []render.Command {
	if !g.hovered {
		return cmds
	}

	fill := render.RGBA(1, 1, 1, 0.08)
	stroke := render.RGBA(1, 1, 1, 0.25)
	if _, overUnit := g.unitAt(g.hoverI, g.hoverJ); overUnit {
		fill = render.RGBA(1, 1, 1, 0.16)
		stroke = render.RGBA(1, 1, 1, 0.5)
	} else if g.State.Selected >= 0 && terrainAt(g.hoverI, g.hoverJ) == terrainGrass {
		fill = render.RGBA(0.5, 0.95, 0.5, 0.28)
		stroke = render.RGBA(0.6, 1, 0.6, 0.7)
	}

	cmds = append(cmds, render.DrawRect{
		Rect:        inset(tileRect(g.hoverI, g.hoverJ), 0.4),
		Fill:        fill,
		Stroke:      stroke,
		StrokeWidth: 0.3,
		BaseCommand: render.BaseCommand{LayerValue: layerHover},
	})
	return cmds
}

// appendUnitGround draws what sits under each unit: a contact shadow while it
// stands (the engine's projected shadow takes over the moment it lifts off)
// and the selection ring. Both are ground marks, so layerMarks is all the
// ordering they need: anything standing anywhere correctly draws over them.
func (g *Game) appendUnitGround(cmds []render.Command, ctx *runtime.RenderContext) []render.Command {
	for u := range g.State.Units {
		transform, ok := ctx.World.Transform(g.units[u])
		if !ok {
			continue
		}
		ground := transform.Position

		height, _ := ctx.World.Height(g.units[u])
		if height.Z <= 0.01 {
			cmds = append(cmds, render.DrawEllipse{
				Center:      ground.Add(math2d.Vec2{Y: 0.6}),
				RadiusX:     2.4,
				RadiusY:     2.4,
				Fill:        standShadow,
				BaseCommand: render.BaseCommand{LayerValue: layerMarks},
			})
		}

		if g.State.Selected == u {
			pulse := 0.55 + 0.25*math.Sin(ctx.Time*5)
			cmds = append(cmds, render.DrawEllipse{
				Center:      ground,
				RadiusX:     3.1,
				RadiusY:     3.1,
				Stroke:      render.RGBA(1, 0.85, 0.35, pulse),
				StrokeWidth: 0.35,
				BaseCommand: render.BaseCommand{LayerValue: layerMarks},
			})
		}
	}
	return cmds
}

func darken(c render.Color) render.Color {
	return render.RGBA(c.R*0.55, c.G*0.55, c.B*0.55, c.A)
}

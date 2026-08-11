package outpost

import (
	"embed"
	"fmt"
	"io/fs"
	"math"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/tilemap"
)

//go:embed assets
var embeddedAssets embed.FS

const (
	viewportWidth   = 1600
	viewportHeight  = 960
	groundBatchBase = render.SpriteBatchID(100)

	// cameraAir is the margin FrameGround adds around the board: enough for
	// the peaks above it and the cliffs below without dwarfing the island.
	cameraAir = 0.12

	unitCount = 2
)

var (
	sheepIdleSrc = math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: 124, Y: 62},
	)
	sheepAirborneSrc = math2d.NewRect(
		math2d.Vec2{X: 124, Y: 0},
		math2d.Vec2{X: 124, Y: 62},
	)
	sheepLandingSrc = math2d.NewRect(
		math2d.Vec2{X: 0, Y: 62},
		math2d.Vec2{X: 124, Y: 62},
	)
)

// isoView is the entire projection: screenX = x - y, screenY = (x + y) / 2,
// the classic 2:1 dimetric view. The simulation below never mentions it
// again -- tiles stay squares, clicks come back through ScreenToWorld.
var isoView = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}

var unitBodies = [unitCount]render.Color{
	render.RGBA8(128, 128, 128, 255),
	render.RGBA8(255, 255, 255, 255),
}

var unitStarts = [unitCount][2]int{{2, 5}, {6, 2}}

// UnitState is which tile a unit claims. The hop itself lives in the world as
// a scene.HopTween, so the flight is simulation state without being repeated
// here.
type UnitState struct {
	TileI, TileJ int
	Airborne     bool
	LandingHeld  bool
}

// CursorState is the gamepad and keyboard tile cursor. The stick hysteresis
// and the repeat clock live inside the state on purpose: they decide which
// tile a confirm acts on, so a replay has to carry them tick for tick.
type CursorState struct {
	I, J int

	// Active flips on the first directional input and stays: a player who
	// never touches pad or arrows never sees the cursor.
	Active bool

	Direction input.StickDPad
	Repeat    input.Repeater
	DX, DY    int // the direction held last tick, to rearm Repeat on change
}

type State struct {
	// Selected is the index of the chosen unit, -1 for none. It is game
	// state, not decoration: a click means something different because of it,
	// so a replay has to carry it.
	Selected int
	Units    [unitCount]UnitState
	Cursor   CursorState
}

type Game struct {
	runtime.StateSlot[State]

	units [unitCount]scene.EntityID

	// Presentation-only, rebuilt or refreshed without entering State.
	static          []render.Command
	projection      render.Projection
	ground          tilemap.Layer
	groundAtlas     tilemap.Atlas
	groundBuffer    []render.SpriteInstance
	terrainUploaded bool
	hoverI          int
	hoverJ          int
	hovered         bool

	commands []render.Command

	assets asset.Manifest
}

func NewGame() (*Game, error) {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	m, err := asset.Load(sub)
	if err != nil {
		return nil, err
	}
	ground, err := tilemap.ParseASCII(boardMap[:], groundLegend)
	if err != nil {
		return nil, fmt.Errorf("parse outpost ground: %w", err)
	}
	ground.Origin = math2d.Vec2{}
	ground.CellSize = math2d.Vec2{X: tileSize, Y: tileSize}
	return &Game{
		assets: m,
		ground: ground,
		groundAtlas: tilemap.Atlas{
			Texture:  m.Texture("terrain_atlas"),
			TileSize: math2d.Vec2{X: terrainAtlasCell, Y: terrainAtlasCell},
			Columns:  terrainAtlasColumns,
		},
	}, nil
}

func (g *Game) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			// Camera.World frames *view* space -- the board's diamond, not its
			// square -- and FrameGround does that projection so the framing is
			// "the board plus some air" instead of four numbers found by eye.
			World: render.FrameGround(isoView, boardRect(), cameraAir),
			Viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: viewportWidth, Y: viewportHeight},
			),
			Aspect: render.AspectFit,
			View:   isoView,
		},
		Clear:                render.RGBA8(24, 26, 40, 255),
		InputBindings:        cursorBindings(),
		SimulationHz:         60,
		FPS:                  60,
		HistoryLen:           600,
		DisableDefaultSplash: true,
		Assets:               g.assets,
	}
}

// cursorBindings maps every way of steering the tile cursor onto the same two
// axes, plus confirm. The stick arrives analog and the buttons arrive as ±1;
// StickDPad quantizes both the same way, so the game never asks which device
// the player picked up.
func cursorBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Gamepad(input.GamepadLeftStickX), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Gamepad(input.GamepadLeftStickY), Action: input.ActionMoveY, Scale: 1},
		{Source: input.Gamepad(input.GamepadDpadRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Gamepad(input.GamepadDpadLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Gamepad(input.GamepadDpadDown), Action: input.ActionMoveY, Scale: 1},
		{Source: input.Gamepad(input.GamepadDpadUp), Action: input.ActionMoveY, Scale: -1},
		{Source: input.Gamepad(input.GamepadA), Action: input.ActionConfirm, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowDown), Action: input.ActionMoveY, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowUp), Action: input.ActionMoveY, Scale: -1},
		{Source: input.Keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: 1},
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

	// Lighting is declarative and render-only, like the projection: the world
	// says which lights exist and which layers feel them, and no checksum ever
	// sees any of it. The dusk ambient dims the actor layer into the evening
	// the dark sky and lit windows already suggest; the plaza lantern pays it
	// back with a warm pool. Everything standing on the board — units, trees,
	// mountains, buildings — shares that layer and dims together; the static
	// terrain lives outside the world and keeps its authored daylight, and
	// marks, hover and clouds stay out of the band.
	ctx.World.SetAmbientLight(scene.AmbientLight{Color: duskAmbient})
	ctx.World.SetLitLayers(layerActors, layerActors)

	// The projection is the whole 2.5D vocabulary in one value: every piece
	// of fake volume below -- cliffs, mountains, buildings -- extrudes along
	// its LiftVector, the same vector that lifts entities with Height, so
	// authored geometry and engine-lifted bodies can never disagree about
	// which way up is.
	g.projection = projection

	g.publishAllGround(ctx.Batches)
	g.static = buildStaticBoard(projection)
	spawnProps(ctx.World, projection)

	g.State = State{
		Selected: -1,
		// The cursor waits, inactive, on the first unit's tile: summoning it
		// starts somewhere meaningful instead of a corner.
		Cursor: CursorState{I: unitStarts[0][0], J: unitStarts[0][1]},
	}
	sheepTexture := ctx.Assets.Texture("sheep")
	for u := range g.units {
		i, j := unitStarts[u][0], unitStarts[u][1]
		g.State.Units[u] = UnitState{TileI: i, TileJ: j}
		g.units[u] = ctx.World.SpawnBundle(scene.Bundle{
			Transform: &scene.Transform{Position: tileCenter(i, j)},
			ImageSprite: &scene.ImageSprite{
				BaseSprite: scene.BaseSprite{
					Layer: layerActors,
					Color: unitBodies[u],
					Plane: scene.PlaneUpright,
				},
				Texture:         sheepTexture,
				Size:            math2d.Vec2{X: 12, Y: 6},
				ShadowFootprint: math2d.Vec2{X: 1.8, Y: 1.8},
				Src:             math2d.Rect{Min: math2d.Vec2{X: 0, Y: 0}, Max: math2d.Vec2{X: 124, Y: 62}},
			},
			Height: &scene.Height{},
		})

		// The stand shadow is the engine's contact shadow: ShadowStyle opts
		// in, and the zero value takes the projection's light-derived look,
		// so standing and airborne shadows come from one source.
		ctx.World.SetShadowStyle(g.units[u], scene.ShadowStyle{Scale: 0.5, Offset: math2d.Vec2{X: 0.7, Y: -0.3}})

		ctx.World.SpawnAttached(g.units[u], math2d.Vec2{X: -0.7, Y: -0.7}, 0, scene.Bundle{
			CircleSprite: &scene.CircleSprite{
				Radius:     0.6,
				BaseSprite: scene.BaseSprite{Layer: layerActors, Color: render.RGBA(1, 1, 1, 0.35)},
			},
		})
	}
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	g.trackHover(ctx)
	g.handleClicks(ctx)
	g.steerCursor(ctx, dt)
	g.publishDirtyGround(ctx.Batches)
}

func (g *Game) PostStep(ctx *runtime.UpdateContext, _ scene.StepStats) {
	for u, id := range g.units {
		state := &g.State.Units[u]

		_, airborne := ctx.World.HopTween(id)

		if state.Airborne && !airborne {
			state.LandingHeld = true
		}

		state.Airborne = airborne

		switch {
		case state.LandingHeld:
			g.setSheepFrame(ctx, id, sheepLandingSrc)
		case airborne:
			g.setSheepFrame(ctx, id, sheepAirborneSrc)
		default:
			g.setSheepFrame(ctx, id, sheepIdleSrc)
		}
	}
}

// steerCursor is the whole gamepad/keyboard scheme: the Mapper already merged
// stick, dpad and arrows into two axes, StickDPad turns them into a held
// direction, and the Repeater decides when a held direction steps again.
func (g *Game) steerCursor(ctx *runtime.UpdateContext, dt float64) {
	cursor := &g.State.Cursor

	dx, dy := cursor.Direction.Update(
		ctx.Input.Value(input.ActionMoveX),
		ctx.Input.Value(input.ActionMoveY),
	)
	if dx != cursor.DX || dy != cursor.DY {
		// A new direction steps right away instead of waiting out a delay
		// armed for the old one.
		cursor.Repeat = input.Repeater{}
		cursor.DX, cursor.DY = dx, dy
	}

	for range cursor.Repeat.Step(dx != 0 || dy != 0, dt) {
		if !cursor.Active {
			cursor.Active = true
			continue // the first press summons the cursor where it waits
		}
		cursor.I = min(max(cursor.I+dx, 0), boardCols-1)
		cursor.J = min(max(cursor.J+dy, 0), boardRows-1)
	}

	if cursor.Active && ctx.Input.JustPressed(input.ActionConfirm) {
		g.clickTile(ctx, cursor.I, cursor.J)
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
	for _, click := range ctx.Pointer.Clicks(input.PointerPrimary) {
		if i, j, ok := tileAt(ctx.Camera().ScreenToWorld(click)); ok {
			g.clickTile(ctx, i, j)
		}
	}
}

func (g *Game) clickTile(ctx *runtime.UpdateContext, i, j int) {
	if u, ok := g.unitAt(i, j); ok {
		g.State.Units[u].LandingHeld = false
		g.State.Selected = u
		return
	}

	selected := g.State.Selected
	if selected < 0 || terrainAt(i, j) != terrainGrass {
		return
	}
	if _, hopping := ctx.World.HopTween(g.units[selected]); hopping {
		return
	}

	unit := &g.State.Units[selected]
	from := tileCenter(unit.TileI, unit.TileJ)
	to := tileCenter(i, j)
	duration, apex := hopFlight(from, to)
	ctx.World.Hop(g.units[selected], to, scene.HopOptions{Duration: duration, Apex: apex})
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

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	if !g.terrainUploaded {
		ctx.UpdateTexture(g.groundAtlas.Texture, terrainAtlasImage())
		g.terrainUploaded = true
	}

	cmds := g.appendGroundBatches(g.commands[:0], ctx.Camera())
	cmds = append(cmds, g.static...)
	cmds = g.appendClouds(cmds, ctx.Time)
	cmds = g.appendWater(cmds, ctx.Time)
	cmds = g.appendHover(cmds)
	cmds = g.appendCursor(cmds)
	cmds = g.appendUnitGround(cmds, ctx)

	cmds = ctx.World.RenderCommandsInto(cmds, ctx.Alpha)
	cmds = g.appendConfusion(cmds, ctx)

	g.commands = cmds
	return cmds
}

func (g *Game) appendGroundBatches(cmds []render.Command, cam render.Camera) []render.Command {
	for _, chunk := range tilemap.VisibleChunks(&g.ground, cam.ForLayer(layerGround)) {
		cmds = append(cmds, render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: layerGround},
			Batch:       groundBatchID(chunk),
		})
	}
	return cmds
}

func (g *Game) publishAllGround(batches *runtime.BatchPort) {
	for y := 0; y < g.ground.H; y += tilemap.ChunkSize {
		for x := 0; x < g.ground.W; x += tilemap.ChunkSize {
			g.publishGroundChunk(batches, tilemap.ChunkCoord{X: x / tilemap.ChunkSize, Y: y / tilemap.ChunkSize})
		}
	}
}

func (g *Game) publishDirtyGround(batches *runtime.BatchPort) {
	for _, chunk := range g.ground.DirtyChunks() {
		g.publishGroundChunk(batches, chunk)
	}
}

func (g *Game) publishGroundChunk(batches *runtime.BatchPort, chunk tilemap.ChunkCoord) {
	instances := g.ground.Chunk(chunk.X, chunk.Y, g.groundAtlas, g.groundBuffer)
	g.groundBuffer = instances
	batches.UpdateSprites(groundBatchID(chunk), instances)
}

func groundBatchID(chunk tilemap.ChunkCoord) render.SpriteBatchID {
	return groundBatchBase + render.SpriteBatchID(chunk.Y*256+chunk.X)
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
	for j := range boardRows {
		for i := range boardCols {
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

// appendCursor draws the pad/keyboard tile cursor: the same tile framing as
// the hover, in the selection gold so the two never read as the same thing.
func (g *Game) appendCursor(cmds []render.Command) []render.Command {
	if !g.State.Cursor.Active {
		return cmds
	}
	return append(cmds, render.DrawRect{
		Rect:        inset(tileRect(g.State.Cursor.I, g.State.Cursor.J), 0.4),
		Stroke:      render.RGBA(1, 0.85, 0.35, 0.9),
		StrokeWidth: 0.45,
		BaseCommand: render.BaseCommand{LayerValue: layerHover},
	})
}

// appendUnitGround draws the selection ring under the selected unit. The
// contact shadow that used to live here is the engine's now: ShadowStyle on
// each unit keeps it on the ground and hands it over the moment the unit
// lifts off. The ring is a ground mark, so layerMarks is all the ordering it
// needs: anything standing anywhere correctly draws over it.
func (g *Game) appendUnitGround(cmds []render.Command, ctx *runtime.RenderContext) []render.Command {
	for u := range g.State.Units {
		transform, ok := ctx.World.Transform(g.units[u])
		if !ok {
			continue
		}
		ground := transform.Position

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

func (g *Game) setSheepFrame(
	ctx *runtime.UpdateContext,
	id scene.EntityID,
	frame math2d.Rect,
) {
	sprite, ok := ctx.World.ImageSprite(id)
	if !ok || sprite.Src == frame {
		return
	}

	sprite.Src = frame
	ctx.World.SetImageSprite(id, sprite)
}

func (g *Game) appendConfusion(
	cmds []render.Command,
	ctx *runtime.RenderContext,
) []render.Command {
	for u, state := range g.State.Units {
		if !state.LandingHeld {
			continue
		}

		id := g.units[u]
		transform, ok := ctx.World.Transform(id)
		if !ok {
			continue
		}

		height, _ := ctx.World.Height(id)
		center := g.projection.Lift(transform.Position, height.Z+3.4)

		for star := range 3 {
			angle := ctx.Time*2.5 + float64(star)*2*math.Pi/3
			orbit := math2d.Vec2{
				X: math.Cos(angle) * 0.8,
				Y: math.Sin(angle) * 0.35,
			}

			cmds = append(cmds, render.DrawPolygon{
				Points: starPoints(
					center.Add(orbit),
					0.45,
					0.18,
					angle,
				),
				Fill: render.RGBA8(255, 220, 70, 255),
				BaseCommand: render.BaseCommand{
					LayerValue: layerActors + 1,
				},
			})
		}
	}

	return cmds
}

func starPoints(
	center math2d.Vec2,
	outer, inner, rotation float64,
) []math2d.Vec2 {
	points := make([]math2d.Vec2, 10)

	for i := range points {
		radius := inner
		if i%2 == 0 {
			radius = outer
		}

		angle := rotation + float64(i)*math.Pi/5
		points[i] = center.Add(math2d.Vec2{
			X: math.Cos(angle) * radius,
			Y: math.Sin(angle) * radius,
		})
	}

	return points
}

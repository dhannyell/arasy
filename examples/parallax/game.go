package parallax

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

const (
	viewportWidth  = 1280
	viewportHeight = 720

	worldWidth  = 160.0
	worldHeight = 90.0

	groundLayer     = 0
	playerLayer     = 10
	hudLayer        = 100
	skyLayer        = -20
	moonLayer       = -19
	hillsLayer      = -10
	foregroundLayer = 20
	bushLayer       = 30
	groundSurfaceY  = 48.0

	// The camera starts with this framing; repeated layers use periods that
	// keep their patterns continuous as the camera moves beyond it.
	skyFactor        = 0.0
	hillsFactor      = 0.5 // 0.5*160 = 80, one hillsPeriod
	groundFactor     = 1.0 // the floor follows the camera exactly
	foregroundFactor = 1.2 // 1.2*160 = 192, three foregroundPeriod
	bushFactor       = 1.8 // 1.8*160 = 288, three bushPeriod

	playerStartX       = 80.0
	playerStartY       = groundSurfaceY - playerRadius
	playerRadius       = 2.4
	playerMoveDistance = 100.0
	playerDesiredSpeed = 36.0
	playerAccel        = 130.0
	playerDecel        = 10
	playerAgility      = 72.0

	zoomStart = 1.0
	zoomStep  = 1.1
	zoomMin   = 0.75
	zoomMax   = 1.5

	stripMargin    = 32.0
	hillsPeriod    = 80.0
	hillsBaseY     = groundSurfaceY
	hillsPeakY     = 18.0
	hillsPeakYTwo  = 28.0
	hillsPeakWidth = 26.0

	foregroundPeriod = 64.0
	foregroundTreeX  = 12.0
	foregroundTreeY  = 31.0
	foregroundTreeR  = 8.0
	foregroundPoleX  = 42.0
	foregroundPoleY  = 22.0
	foregroundPoleW  = 2.0
	foregroundPoleH  = 26.0

	// The bushes sit at the bottom edge so they read as centimetres from
	// the lens: they cover the player and slide faster than everything else.
	bushPeriod  = 96.0
	bushOneX    = 18.0
	bushTwoX    = 74.0
	bushOneR    = 13.0
	bushTwoR    = 10.0
	bushCenterY = worldHeight - 4.0

	hudMargin     = 4.0
	hudLineHeight = 4.0
	hudTextSize   = 2.6
	hudSmallSize  = 2.0
)

var (
	skyFill          = render.RGBA8(10, 14, 24, 255)
	moonGlowFill     = render.RGBA(0.65, 0.68, 0.67, 0.10)
	moonFill         = render.RGBA8(202, 205, 198, 255)
	starFill         = render.RGBA8(155, 164, 165, 210)
	hillsFill        = render.RGBA8(35, 42, 52, 255)
	hillsFarFill     = render.RGBA8(25, 31, 40, 255)
	groundBaseFill   = render.RGBA8(61, 66, 68, 255)
	foregroundTrunk  = render.RGBA8(17, 20, 24, 255)
	foregroundLeaves = render.RGBA8(14, 18, 22, 255)
	bushFill         = render.RGBA8(6, 8, 11, 255)
	backgroundTrunk  = render.RGBA8(27, 33, 41, 255)
	backgroundLeaves = render.RGBA8(24, 30, 38, 255)
	playerFill       = render.RGBA8(195, 201, 198, 255)
	playerStroke     = render.RGBA8(235, 237, 228, 255)
	shadowFill       = render.RGBA(0.01, 0.01, 0.02, 0.45)
	hudFill          = render.RGBA8(220, 224, 216, 255)
	hudMuted         = render.RGBA8(142, 151, 153, 255)
)

var parallaxLayers = []render.LayerParallax{
	{FromLayer: skyLayer, ToLayer: moonLayer, Factor: math2d.Vec2{X: skyFactor, Y: skyFactor}, Anchor: math2d.Vec2{Y: worldHeight / 2}},
	{FromLayer: hillsLayer, ToLayer: hillsLayer, Factor: math2d.Vec2{X: hillsFactor, Y: groundFactor}},
	{FromLayer: groundLayer, ToLayer: playerLayer, Factor: math2d.Vec2{X: groundFactor, Y: groundFactor}},
	{FromLayer: foregroundLayer, ToLayer: foregroundLayer, Factor: math2d.Vec2{X: foregroundFactor, Y: groundFactor}},
	{FromLayer: bushLayer, ToLayer: bushLayer, Factor: math2d.Vec2{X: bushFactor, Y: groundFactor}},
}

// Game is a small side-scroller that shows the camera's layer factors.
type Game struct {
	player scene.EntityID
	zoom   float64
}

// NewGame creates the side-scroller demo.
func NewGame() (*Game, error) {
	return &Game{
		zoom: zoomStart,
	}, nil
}

// Setup defines the fixed-step scene and its controls.
func (g *Game) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			Viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: viewportWidth, Y: viewportHeight},
			),
			Aspect:   render.AspectExpand,
			Parallax: parallaxLayers,
		},
		Clear:                skyFill,
		InputBindings:        parallaxBindings(),
		SimulationHz:         60,
		FPS:                  60,
		HistoryLen:           600,
		DisableDefaultSplash: true,
	}
}

func parallaxBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyA), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyPageUp), Action: input.ActionConfirm, Scale: 1},
		{Source: input.Keyboard(input.KeyPageDown), Action: input.ActionCancel, Scale: 1},
	}
}

// Init places the actor on the infinite floor.
func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.player = ctx.World.Spawn()
	ctx.World.SetTransform(g.player, scene.Transform{
		Position: math2d.Vec2{X: playerStartX, Y: playerStartY},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	ctx.World.SetLocomotion(g.player, scene.Locomotion{
		Accel:    playerAccel,
		Decel:    playerDecel,
		MaxSpeed: playerDesiredSpeed,
		Agility:  playerAgility,
	})
}

// Update keeps the actor on the ground and applies the demo zoom actions.
func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	transform, ok := ctx.World.Transform(g.player)
	if !ok {
		return
	}

	direction := math2d.Vec2{
		X: ctx.Input.Value(input.ActionMoveX),
	}
	if direction.X == 0 && direction.Y == 0 {
		ctx.World.RemoveMoveIntent(g.player)
	} else {
		target := transform.Position.Add(direction.Mul(playerMoveDistance))
		target.Y = playerStartY
		ctx.World.SetMoveIntent(g.player, scene.MoveIntent{
			Target:       target,
			DesiredSpeed: playerDesiredSpeed,
		})
	}

	if ctx.Input.JustPressed(input.ActionConfirm) {
		g.zoom = math2d.Clamp(g.zoom*zoomStep, zoomMin, zoomMax)
	}
	if ctx.Input.JustPressed(input.ActionCancel) {
		g.zoom = math2d.Clamp(g.zoom/zoomStep, zoomMin, zoomMax)
	}
}

// Render draws the layer bands through their declared camera factors.
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	playerTransform, _ := ctx.World.Transform(g.player)
	playerPosition := playerTransform.Position
	camera := ctx.Camera().Pan(playerPosition.Sub(ctx.Camera().World.Center()))
	camera = camera.ZoomAt(g.zoom, playerPosition)
	ctx.SetCamera(camera)

	commands := make([]render.Command, 0, 64)
	commands = appendSky(commands, camera)
	commands = appendHills(commands, camera)
	commands = g.appendGround(commands, camera)
	commands = appendPlayer(commands, playerPosition)
	commands = appendForeground(commands, camera)
	commands = appendBushes(commands, camera)
	commands = appendHUD(commands, camera)
	return commands
}

func appendSky(commands []render.Command, camera render.Camera) []render.Command {
	view := camera.ForLayer(skyLayer).World
	return append(commands,
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: skyLayer},
			Rect:        math2d.NewRect(view.Min.Sub(math2d.Vec2{X: stripMargin, Y: stripMargin}), view.Size().Add(math2d.Vec2{X: stripMargin * 2, Y: stripMargin * 2})),
			Fill:        skyFill,
		},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: moonLayer}, Center: math2d.Vec2{X: 30, Y: 10}, Radius: 13, Fill: moonGlowFill},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: moonLayer}, Center: math2d.Vec2{X: 30, Y: 10}, Radius: 7, Fill: moonFill},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: moonLayer}, Center: math2d.Vec2{X: 12, Y: 13}, Radius: 0.35, Fill: starFill},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: moonLayer}, Center: math2d.Vec2{X: 58, Y: 9}, Radius: 0.3, Fill: starFill},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: moonLayer}, Center: math2d.Vec2{X: 112, Y: 18}, Radius: 0.3, Fill: starFill},
	)
}

func appendHills(commands []render.Command, camera render.Camera) []render.Command {
	view := camera.ForLayer(hillsLayer).World
	first, count := render.WrapOffsets(view.Min.X-stripMargin, view.Max.X+stripMargin, hillsPeriod)
	for copyIndex := range count {
		x := first + float64(copyIndex)*hillsPeriod
		// copyIndex is view-local; the world strip keeps its color when the
		// camera crosses a WrapOffsets boundary.
		stripIndex := int(first/hillsPeriod) + copyIndex
		fill := hillsFill
		if stripIndex%2 != 0 {
			fill = hillsFarFill
		}
		commands = append(commands,
			render.DrawPolygon{
				BaseCommand: render.BaseCommand{LayerValue: hillsLayer},
				Points: []math2d.Vec2{
					{X: x, Y: hillsBaseY},
					{X: x + hillsPeakWidth, Y: hillsPeakY},
					{X: x + hillsPeriod*0.5, Y: hillsBaseY},
					{X: x + hillsPeriod*0.75, Y: hillsPeakYTwo},
					{X: x + hillsPeriod, Y: hillsBaseY},
					{X: x + hillsPeriod, Y: worldHeight + stripMargin},
					{X: x, Y: worldHeight + stripMargin},
				},
				Fill: fill,
			},
		)
		for _, treeX := range []float64{22, 57} {
			commands = append(commands,
				render.DrawRect{
					BaseCommand: render.BaseCommand{LayerValue: hillsLayer},
					Rect:        math2d.NewRect(math2d.Vec2{X: x + treeX - 0.8, Y: groundSurfaceY - 14}, math2d.Vec2{X: 1.6, Y: 14}),
					Fill:        backgroundTrunk,
				},
				render.DrawCircle{
					BaseCommand: render.BaseCommand{LayerValue: hillsLayer},
					Center:      math2d.Vec2{X: x + treeX, Y: groundSurfaceY - 17},
					Radius:      5,
					Fill:        backgroundLeaves,
				},
			)
		}
	}
	return commands
}

func (g *Game) appendGround(commands []render.Command, camera render.Camera) []render.Command {
	view := camera.ForLayer(groundLayer).World
	floorBottom := view.Max.Y + stripMargin
	if floorBottom <= groundSurfaceY {
		return commands
	}
	return append(commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: groundLayer},
		Rect: math2d.NewRect(
			math2d.Vec2{X: view.Min.X - stripMargin, Y: groundSurfaceY},
			math2d.Vec2{X: view.Size().X + stripMargin*2, Y: floorBottom - groundSurfaceY},
		),
		Fill: groundBaseFill,
	})
}

func appendPlayer(commands []render.Command, position math2d.Vec2) []render.Command {
	return append(commands,
		render.DrawEllipse{
			BaseCommand: render.BaseCommand{LayerValue: groundLayer},
			Center:      position.Add(math2d.Vec2{Y: playerRadius}),
			RadiusX:     playerRadius * 1.2,
			RadiusY:     playerRadius * 0.35,
			Fill:        shadowFill,
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: playerLayer},
			From:        position.Add(math2d.Vec2{X: -1.1, Y: 0.7}),
			To:          position.Add(math2d.Vec2{X: -0.9, Y: 2.4}),
			Width:       0.65,
			Color:       playerFill,
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: playerLayer},
			From:        position.Add(math2d.Vec2{X: 0.5, Y: 0.7}),
			To:          position.Add(math2d.Vec2{X: 0.8, Y: 2.4}),
			Width:       0.65,
			Color:       playerFill,
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: playerLayer},
			From:        position.Add(math2d.Vec2{X: -1.1, Y: -1.0}),
			To:          position.Add(math2d.Vec2{X: -2.0, Y: 0.3}),
			Width:       0.55,
			Color:       playerFill,
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: playerLayer},
			From:        position.Add(math2d.Vec2{X: 1.1, Y: -1.0}),
			To:          position.Add(math2d.Vec2{X: 1.9, Y: 0.3}),
			Width:       0.55,
			Color:       playerFill,
		},
		render.DrawEllipse{
			BaseCommand: render.BaseCommand{LayerValue: playerLayer},
			Center:      position.Add(math2d.Vec2{Y: -0.7}),
			RadiusX:     1.4,
			RadiusY:     2.1,
			Fill:        playerFill,
			Stroke:      playerStroke,
			StrokeWidth: 0.3,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: playerLayer},
			Center:      position.Add(math2d.Vec2{Y: -3.0}),
			Radius:      1.25,
			Fill:        playerFill,
			Stroke:      playerStroke,
			StrokeWidth: 0.3,
		},
	)
}

func appendForeground(commands []render.Command, camera render.Camera) []render.Command {
	view := camera.ForLayer(foregroundLayer).World
	first, count := render.WrapOffsets(view.Min.X-stripMargin, view.Max.X+stripMargin, foregroundPeriod)
	for copyIndex := range count {
		x := first + float64(copyIndex)*foregroundPeriod
		treeX := x + foregroundTreeX
		commands = append(commands,
			render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: foregroundLayer},
				Rect:        math2d.NewRect(math2d.Vec2{X: treeX - 1, Y: foregroundTreeY + 5}, math2d.Vec2{X: 2, Y: groundSurfaceY - foregroundTreeY - 5}),
				Fill:        foregroundTrunk,
			},
			render.DrawCircle{
				BaseCommand: render.BaseCommand{LayerValue: foregroundLayer},
				Center:      math2d.Vec2{X: treeX, Y: foregroundTreeY},
				Radius:      foregroundTreeR,
				Fill:        foregroundLeaves,
			},
			render.DrawCircle{
				BaseCommand: render.BaseCommand{LayerValue: foregroundLayer},
				Center:      math2d.Vec2{X: treeX - 4, Y: foregroundTreeY + 2},
				Radius:      foregroundTreeR * 0.65,
				Fill:        foregroundLeaves,
			},
			render.DrawCircle{
				BaseCommand: render.BaseCommand{LayerValue: foregroundLayer},
				Center:      math2d.Vec2{X: treeX + 4, Y: foregroundTreeY + 2},
				Radius:      foregroundTreeR * 0.65,
				Fill:        foregroundLeaves,
			},
			render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: foregroundLayer},
				Rect:        math2d.NewRect(math2d.Vec2{X: x + foregroundPoleX, Y: foregroundPoleY}, math2d.Vec2{X: foregroundPoleW, Y: foregroundPoleH}),
				Fill:        foregroundTrunk,
			},
		)
	}
	return commands
}

func appendBushes(commands []render.Command, camera render.Camera) []render.Command {
	view := camera.ForLayer(bushLayer).World
	first, count := render.WrapOffsets(view.Min.X-stripMargin, view.Max.X+stripMargin, bushPeriod)
	for copyIndex := range count {
		x := first + float64(copyIndex)*bushPeriod
		for _, bush := range []struct {
			offset, radius float64
		}{
			{offset: bushOneX, radius: bushOneR},
			{offset: bushTwoX, radius: bushTwoR},
		} {
			center := math2d.Vec2{X: x + bush.offset, Y: bushCenterY}
			commands = append(commands,
				render.DrawCircle{
					BaseCommand: render.BaseCommand{LayerValue: bushLayer},
					Center:      center,
					Radius:      bush.radius,
					Fill:        bushFill,
				},
				render.DrawCircle{
					BaseCommand: render.BaseCommand{LayerValue: bushLayer},
					Center:      center.Add(math2d.Vec2{X: -bush.radius * 0.7, Y: bush.radius * 0.3}),
					Radius:      bush.radius * 0.7,
					Fill:        bushFill,
				},
				render.DrawCircle{
					BaseCommand: render.BaseCommand{LayerValue: bushLayer},
					Center:      center.Add(math2d.Vec2{X: bush.radius * 0.7, Y: bush.radius * 0.3}),
					Radius:      bush.radius * 0.7,
					Fill:        bushFill,
				},
			)
		}
	}
	return commands
}

func appendHUD(commands []render.Command, camera render.Camera) []render.Command {
	position := camera.ScreenToWorld(camera.Viewport.Min.Add(math2d.Vec2{X: hudMargin, Y: hudMargin}))
	return append(commands,
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: hudLayer},
			Position:    position,
			Text:        "PARALLAX   A/D OR ARROWS: MOVE   PAGEUP/PAGEDOWN: ZOOM",
			Color:       hudFill,
			Size:        hudTextSize,
			Baseline:    render.BaselineTop,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: hudLayer},
			Position:    position.Add(math2d.Vec2{Y: hudLineHeight}),
			Text:        "MOON 0.0   HILLS 0.5   GROUND 1.0   FOREGROUND 1.2   BUSH 1.8",
			Color:       hudMuted,
			Size:        hudSmallSize,
			Baseline:    render.BaselineTop,
		},
	)
}

package parallax

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"io/fs"

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
	viewportWidth  = 1280
	viewportHeight = 720

	worldWidth  = 160.0
	worldHeight = 90.0

	tileSize          = 4.0
	groundColumns     = int(worldWidth / tileSize)
	groundRows        = 30
	groundLayer       = 0
	playerLayer       = 10
	hudLayer          = 100
	skyLayer          = -20
	moonLayer         = -19
	hillsLayer        = -10
	foregroundLayer   = 20
	groundBatchBase   = render.SpriteBatchID(100)
	groundAtlasCell   = 16
	groundAtlasCols   = 2
	groundAtlasWidth  = groundAtlasCell * groundAtlasCols
	groundAtlasHeight = groundAtlasCell
	groundStartRow    = 12
	groundSurfaceY    = float64(groundStartRow) * tileSize

	skyFactor        = 0.0
	hillsFactor      = 0.3
	groundFactor     = 1.0
	foregroundFactor = 1.3

	playerStartX       = 80.0
	playerStartY       = groundSurfaceY - playerRadius
	playerRadius       = 2.4
	playerMoveDistance = 100.0
	playerDesiredSpeed = 18.0
	playerAccel        = 56.0
	playerDecel        = 72.0
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
}

var groundAtlasFills = [groundAtlasCols]color.RGBA{
	{R: 61, G: 66, B: 68, A: 255},
	{R: 62, G: 67, B: 69, A: 255},
}

var groundAtlasStroke = color.RGBA{R: 61, G: 66, B: 68, A: 255}

var groundRowsASCII = []string{
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"........................................",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggg",
}

// Game is a small side-scroller that shows the camera's layer factors.
type Game struct {
	player scene.EntityID
	ground tilemap.Layer
	zoom   float64
	assets asset.Manifest

	groundAtlas    tilemap.Atlas
	groundBuffer   []render.SpriteInstance
	groundUploaded bool
}

// NewGame loads the side-scroller assets and its tilemap ground.
func NewGame() (*Game, error) {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	assets, err := asset.Load(sub)
	if err != nil {
		return nil, err
	}

	ground, err := tilemap.ParseASCII(groundRowsASCII, map[rune]int16{'g': 0, '.': -1})
	if err != nil {
		return nil, fmt.Errorf("parse parallax ground: %w", err)
	}
	ground.CellSize = math2d.Vec2{X: tileSize, Y: tileSize}
	for y := 0; y < ground.H; y++ {
		for x := 0; x < ground.W; x++ {
			index := y*ground.W + x
			if ground.Tiles[index] >= 0 {
				ground.Tiles[index] = int16((x + y) % groundAtlasCols)
			}
		}
	}

	return &Game{
		ground: ground,
		zoom:   zoomStart,
		assets: assets,
		groundAtlas: tilemap.Atlas{
			Texture:  assets.Texture("ground_atlas"),
			TileSize: math2d.Vec2{X: groundAtlasCell, Y: groundAtlasCell},
			Columns:  groundAtlasCols,
		},
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
			Aspect:   render.AspectFit,
			Parallax: parallaxLayers,
		},
		Clear:                skyFill,
		InputBindings:        parallaxBindings(),
		SimulationHz:         60,
		FPS:                  60,
		HistoryLen:           600,
		DisableDefaultSplash: true,
		Assets:               g.assets,
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

// Init places the actor on the ground and publishes the static terrain.
func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.publishGround(ctx.Batches)

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
		target.X = math2d.Clamp(target.X, 0, worldWidth)
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
	if !g.groundUploaded {
		ctx.UpdateTexture(g.groundAtlas.Texture, groundAtlasImage())
		g.groundUploaded = true
	}

	playerTransform, _ := ctx.World.Transform(g.player)
	playerPosition := playerTransform.Position
	camera := ctx.Camera().Pan(playerPosition.Sub(ctx.Camera().World.Center()))
	camera = camera.ZoomAt(g.zoom, playerPosition)
	ctx.SetCamera(camera)

	commands := make([]render.Command, 0, groundColumns*groundRows)
	commands = appendSky(commands, camera)
	commands = appendHills(commands, camera)
	commands = g.appendGround(commands, camera)
	commands = appendPlayer(commands, playerPosition)
	commands = appendForeground(commands, camera)
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
		fill := hillsFill
		if copyIndex%2 == 0 {
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
	visible := tilemap.VisibleChunks(&g.ground, camera.ForLayer(groundLayer))
	if len(visible) == 0 {
		return commands
	}
	commands = append(commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: groundLayer},
		Rect: math2d.NewRect(
			g.ground.Origin.Add(math2d.Vec2{Y: float64(groundStartRow) * tileSize}),
			math2d.Vec2{X: float64(g.ground.W) * tileSize, Y: float64(g.ground.H-groundStartRow) * tileSize},
		),
		Fill: groundBaseFill,
	})
	for _, chunk := range visible {
		commands = append(commands, render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: groundLayer},
			Batch:       groundBatchID(chunk),
		})
	}
	return commands
}

func (g *Game) publishGround(batches *runtime.BatchPort) {
	for y := 0; y < g.ground.H; y += tilemap.ChunkSize {
		for x := 0; x < g.ground.W; x += tilemap.ChunkSize {
			chunk := tilemap.ChunkCoord{X: x / tilemap.ChunkSize, Y: y / tilemap.ChunkSize}
			instances := g.ground.Chunk(chunk.X, chunk.Y, g.groundAtlas, g.groundBuffer)
			g.groundBuffer = instances
			batches.UpdateSprites(groundBatchID(chunk), instances)
		}
	}
}

func groundBatchID(chunk tilemap.ChunkCoord) render.SpriteBatchID {
	return groundBatchBase + render.SpriteBatchID(chunk.Y*256+chunk.X)
}

func groundAtlasImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, groundAtlasWidth, groundAtlasHeight))
	for tile := range groundAtlasCols {
		fill := groundAtlasFills[tile]
		for y := 0; y < groundAtlasCell; y++ {
			for x := 0; x < groundAtlasCell; x++ {
				pixel := fill
				if x == 0 || y == 0 || x == groundAtlasCell-1 || y == groundAtlasCell-1 {
					pixel = groundAtlasStroke
				}
				img.SetRGBA(tile*groundAtlasCell+x, y, pixel)
			}
		}
	}
	return img
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
			Text:        "MOON 0.0   HILLS 0.3   GROUND 1.0   FOREGROUND 1.3",
			Color:       hudMuted,
			Size:        hudSmallSize,
			Baseline:    render.BaselineTop,
		},
	)
}

package twenty48

import (
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
	"golang.org/x/image/font/gofont/goregular"
)

type phase int

const (
	phaseMenu phase = iota
	phasePlaying
	phaseWon
	phaseGameOver
)

const (
	viewportSize          = 720.0
	worldSize             = 100.0
	boardMarginX          = 10.0
	boardTop              = boardMarginX
	boardPad              = 3.0
	cellGap               = 2.0
	rngSeed               = 0x2048_0607
	animationTicks uint64 = 10
)

var demoAssets = loadDemoAssets()

type matchState struct {
	board     board
	score     uint64
	phase     phase
	continued bool
	animation turnAnimation
}

// turnAnimation is kept by value inside matchState so a rewind or replay
// restores an in-flight turn as well as the final board it is headed toward.
type turnAnimation struct {
	Active      bool
	StartTick   uint64
	Motions     [boardCellCount]tileMotion
	MotionCount int
	Merges      [boardCellCount]bool
	HasSpawn    bool
	SpawnIndex  int
}

type Game2048 struct {
	state runtime.StateSlot[matchState]
	best  uint64
}

func (g *Game2048) GameState() runtime.GameState { return g.state.GameState() }

func (g *Game2048) RestoreGameState(state runtime.GameState) { g.state.RestoreGameState(state) }

func (g *Game2048) Setup() runtime.Config {
	theme := ui.DefaultTheme()
	theme.Font = demoAssets.Font("go-regular")

	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldSize, Y: worldSize}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: viewportSize, Y: viewportSize}),
			Aspect:   render.AspectFit,
		},
		Clear:         render.RGBA(1, 1, 1, 1),
		Bounds:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldSize, Y: worldSize}),
		SimulationHz:  60,
		FPS:           60,
		InputBindings: twenty48Bindings(),
		HistoryLen:    1000,
		Assets:        demoAssets,
		UITheme:       theme,
		Seed:          rngSeed,
	}
}

func (g *Game2048) Init(ctx *runtime.UpdateContext) {
	g.state.State = matchState{phase: phaseMenu}
	spawnTile(ctx, &g.state.State)
	spawnTile(ctx, &g.state.State)
}

func (g *Game2048) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()

	if g.state.State.phase == phasePlaying && ctx.UI.ClickedIn("btn_new", hudNewRect(), ui.SpaceRef) {
		g.restart(ctx)
		return
	}

	if g.updateAnimation(ctx) {
		return
	}

	switch g.state.State.phase {
	case phaseMenu, phaseGameOver, phaseWon:
		g.handleOverlayClicks(ctx)
		return
	}

	if ctx.Input.JustPressed(input.ActionMoveX) {
		if v := ctx.Input.Value(input.ActionMoveX); v < 0 {
			g.tryMove(ctx, dirLeft)
			return
		} else if v > 0 {
			g.tryMove(ctx, dirRight)
			return
		}
	}
	if ctx.Input.JustPressed(input.ActionMoveY) {
		if v := ctx.Input.Value(input.ActionMoveY); v < 0 {
			g.tryMove(ctx, dirUp)
		} else if v > 0 {
			g.tryMove(ctx, dirDown)
		}
	}
}

func (g *Game2048) Render(ctx *runtime.RenderContext) []render.Command {
	cmds := make([]render.Command, 0, 1+gridSize*gridSize*2+8)
	cmds = append(cmds, boardBackgroundCommands()...)
	cmds = append(cmds, tileCommands(g.state.State.board, g.state.State.animation, animationProgress(g.state.State.animation, ctx.Tick, ctx.Alpha))...)
	g.drawHUD(ctx)

	if g.state.State.animation.Active {
		return cmds
	}

	switch g.state.State.phase {
	case phaseMenu:
		g.drawMenuOverlay(ctx)
	case phaseGameOver:
		g.drawGameOverOverlay(ctx)
	case phaseWon:
		g.drawWonOverlay(ctx)
	}

	return cmds
}

func (g *Game2048) tryMove(ctx *runtime.UpdateContext, dir direction) {
	next, gained, moved, trace := g.state.State.board.moveWithTrace(dir)
	if !moved {
		return
	}

	g.state.State.board = next
	g.state.State.score += gained
	if g.state.State.score > g.best {
		g.best = g.state.State.score
	}

	spawnIndex, spawned := spawnTile(ctx, &g.state.State)
	g.state.State.animation = turnAnimation{
		Active:      true,
		StartTick:   ctx.Tick + 1,
		Motions:     trace.Motions,
		MotionCount: trace.Count,
		Merges:      trace.Merges,
		HasSpawn:    spawned,
		SpawnIndex:  spawnIndex,
	}

	if g.state.State.board.hasValue(2048) && !g.state.State.continued {
		g.state.State.phase = phaseWon
	} else if !g.state.State.board.canMove() {
		g.state.State.phase = phaseGameOver
	}
}

func (g *Game2048) updateAnimation(ctx *runtime.UpdateContext) bool {
	animation := g.state.State.animation
	if !animation.Active {
		return false
	}
	if ctx.Tick < animation.StartTick+animationTicks {
		return true
	}

	g.state.State.animation = turnAnimation{}
	return true
}

func (g *Game2048) handleOverlayClicks(ctx *runtime.UpdateContext) {
	switch g.state.State.phase {
	case phaseMenu:
		if ctx.UI.ClickedIn("btn_play", overlayPrimaryButtonRect(), ui.SpaceRef) {
			g.restart(ctx)
		}
	case phaseGameOver:
		if ctx.UI.ClickedIn("btn_again", overlayPrimaryButtonRect(), ui.SpaceRef) {
			g.restart(ctx)
		}
	case phaseWon:
		if ctx.UI.ClickedIn("btn_continue", overlayPrimaryButtonRect(), ui.SpaceRef) {
			g.state.State.phase = phasePlaying
			g.state.State.continued = true
			return
		}
		if ctx.UI.ClickedIn("btn_new_game", overlaySecondaryButtonRect(), ui.SpaceRef) {
			g.restart(ctx)
		}
	}
}

func (g *Game2048) restart(ctx *runtime.UpdateContext) {
	g.state.State = matchState{phase: phasePlaying}
	spawnTile(ctx, &g.state.State)
	spawnTile(ctx, &g.state.State)
}

func spawnTile(ctx *runtime.UpdateContext, state *matchState) (int, bool) {
	empties := state.board.emptyCells()
	if len(empties) == 0 {
		return 0, false
	}
	idx := empties[ctx.Rand.IntN(len(empties))]
	value := uint64(4)
	if ctx.Rand.IntN(10) < 9 {
		value = 2
	}
	state.board[idx] = value
	return idx, true
}

func loadDemoAssets() asset.Manifest {
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json":        &fstest.MapFile{Data: []byte(`{"version":1,"fonts":[{"name":"go-regular","file":"fonts/Go-Regular.ttf"}]}`)},
		"fonts/Go-Regular.ttf": &fstest.MapFile{Data: goregular.TTF},
	})
	if err != nil {
		panic("load 2048 font manifest: " + err.Error())
	}
	return manifest
}

func twenty48Bindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyA), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyW), Action: input.ActionMoveY, Scale: -1},
		{Source: input.Keyboard(input.KeyS), Action: input.ActionMoveY, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowUp), Action: input.ActionMoveY, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowDown), Action: input.ActionMoveY, Scale: 1},
	}
}

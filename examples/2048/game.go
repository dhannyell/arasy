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
	boardMarginX          = 13.0
	boardTop              = 10.0
	boardPad              = 3.0
	cellGap               = 2.0
	rngSeed               = 0x2048_0607
	animationTicks uint64 = 10
	powersBarY            = 640.0
	dragThreshold         = 3.0
	undoCharges           = 3
)

var demoAssets = loadDemoAssets()

type matchState struct {
	board     board
	score     uint64
	phase     phase
	continued bool
	animation turnAnimation
	drag      dragState
	rand      runtime.Rand
	undo      undoState
}

type turnSnapshot struct {
	Board     board
	Score     uint64
	Phase     phase
	Continued bool
	Rand      runtime.Rand
}

type undoState struct {
	Stack [undoCharges]turnSnapshot
	Depth int
	Left  int
}

// dragState is gameplay state rather than transient host state: a drag can
// begin on one tick and finish on another, including across a rewind.
type dragState struct {
	Active bool
	Start  math2d.Vec2
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
		FPS:           59,
		InputBindings: twenty48Bindings(),
		HistoryLen:    1000,
		Assets:        demoAssets,
		UITheme:       theme,
		Seed:          rngSeed,
	}
}

func (g *Game2048) Init(ctx *runtime.UpdateContext) {
	g.newMatch(ctx, phaseMenu)
}

func (g *Game2048) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()

	g.expireAnimation(ctx)

	if g.state.State.phase == phasePlaying && ctx.UI.In(ui.SpaceRef).Clicked("btn_new", hudNewRect()) {
		g.restart(ctx)
		return
	}

	switch g.state.State.phase {
	case phaseMenu, phaseGameOver, phaseWon:
		// The overlay stays hidden while the final turn animation plays out,
		// so clicks must not reach its buttons until it clears.
		if g.state.State.animation.Active {
			return
		}
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

	if g.state.State.undo.usable() &&
		(ctx.UI.In(ui.SpaceRef).Clicked("btn_undo", powerButtonRect(0, 1)) ||
			ctx.Input.JustPressed(input.ActionCancel)) {
		g.tryUndo()
		return
	}

	g.handlePointerDrag(ctx)
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

	g.pushUndo()

	g.state.State.board = next
	g.state.State.score += gained
	if g.state.State.score > g.best {
		g.best = g.state.State.score
	}

	spawnIndex, spawned := spawnTile(&g.state.State)
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

func (g *Game2048) pushUndo() {
	s := &g.state.State
	snapshot := turnSnapshot{Board: s.board, Score: s.score, Phase: s.phase, Continued: s.continued, Rand: s.rand}
	if s.undo.Depth == len(s.undo.Stack) {
		copy(s.undo.Stack[:], s.undo.Stack[1:])
		s.undo.Depth--
	}
	s.undo.Stack[s.undo.Depth] = snapshot
	s.undo.Depth++
}

func (g *Game2048) tryUndo() {
	s := &g.state.State
	if !s.undo.usable() {
		return
	}
	s.undo.Depth--
	s.undo.Left--
	top := s.undo.Stack[s.undo.Depth]
	s.board, s.score, s.phase, s.continued, s.rand =
		top.Board, top.Score, top.Phase, top.Continued, top.Rand
	s.animation = turnAnimation{}
}

// handlePointerDrag accepts a primary-button drag that starts on the board.
// It deliberately reads PointerFrame instead of mapped actions: keyboard
// arrows remain directional buttons, while a mouse drag is a pointer gesture.
func (g *Game2048) handlePointerDrag(ctx *runtime.UpdateContext) {
	for _, event := range ctx.Pointer.Events {
		if event.Button != input.PointerPrimary {
			continue
		}

		switch event.Kind {
		case input.EventButtonDown:
			start := ctx.Camera().ScreenToWorld(event.Pos)
			g.state.State.drag = dragState{
				Active: boardRect().Contains(start),
				Start:  start,
			}
		case input.EventButtonUp:
			g.finishPointerDrag(ctx, event.Pos)
			g.state.State.drag = dragState{}
		}
	}

	if g.state.State.drag.Active && ctx.Pointer.Buttons[input.PointerPrimary] {
		g.finishPointerDrag(ctx, ctx.Pointer.Pos)
	}
}

func (g *Game2048) finishPointerDrag(ctx *runtime.UpdateContext, screenEnd math2d.Vec2) {
	drag := g.state.State.drag
	if !drag.Active {
		return
	}

	end := ctx.Camera().ScreenToWorld(screenEnd)
	delta := end.Sub(drag.Start)
	if delta.LenSq() < dragThreshold*dragThreshold {
		return
	}

	g.state.State.drag = dragState{}
	if abs(delta.X) > abs(delta.Y) {
		if delta.X < 0 {
			g.tryMove(ctx, dirLeft)
			return
		}
		g.tryMove(ctx, dirRight)
		return
	}
	if delta.Y < 0 {
		g.tryMove(ctx, dirUp)
		return
	}
	g.tryMove(ctx, dirDown)
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// expireAnimation clears a finished turn animation. It never gates input:
// a new move during an in-flight animation simply replaces it in tryMove,
// so the tiles snap to their destinations and the new slide starts — the
// same feel as the original 2048, where input is never dropped or delayed.
func (g *Game2048) expireAnimation(ctx *runtime.UpdateContext) {
	animation := g.state.State.animation
	if animation.Active && ctx.Tick >= animation.StartTick+animationTicks {
		g.state.State.animation = turnAnimation{}
	}
}

func (g *Game2048) handleOverlayClicks(ctx *runtime.UpdateContext) {
	in := ctx.UI.In(ui.SpaceRef)
	switch g.state.State.phase {
	case phaseMenu:
		if in.Clicked("btn_play", overlayPrimaryButtonRect()) {
			g.restart(ctx)
		}
	case phaseGameOver:
		if in.Clicked("btn_again", overlayPrimaryButtonRect()) {
			g.restart(ctx)
		}
	case phaseWon:
		if in.Clicked("btn_continue", overlayPrimaryButtonRect()) {
			g.state.State.phase = phasePlaying
			g.state.State.continued = true
			return
		}
		if in.Clicked("btn_new_game", overlaySecondaryButtonRect()) {
			g.restart(ctx)
		}
	}
}

func (u undoState) usable() bool { return u.Left > 0 && u.Depth > 0 }

func (g *Game2048) newMatch(ctx *runtime.UpdateContext, p phase) {
	g.state.State = matchState{
		phase: p,
		rand:  runtime.NewRand(ctx.Rand.Uint64()),
		undo:  undoState{Left: undoCharges},
	}
	spawnTile(&g.state.State)
	spawnTile(&g.state.State)
}

func (g *Game2048) restart(ctx *runtime.UpdateContext) {
	g.newMatch(ctx, phasePlaying)
}

func spawnTile(state *matchState) (int, bool) {
	empties := state.board.emptyCells()
	if len(empties) == 0 {
		return 0, false
	}
	idx := empties[state.rand.IntN(len(empties))]
	value := uint64(4)
	if state.rand.IntN(10) < 9 {
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

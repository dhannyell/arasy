package runtime

import (
	"log/slog"
	"math"
	"time"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

const defaultSimulationHz = 60
const defaultFPS = 60

type Frame struct {
	Camera   render.Camera
	Commands []render.Command
}

type Driver struct {
	game     Game
	cfg      Config
	world    *scene.World
	recorder *recorder
	input    *driverInput
	clock    *clock
	controls devControls
	tick     uint64
	log      *slog.Logger

	// UI
	viewport math2d.Rect
	uiState  ui.State
	uiTheme  ui.Theme

	pendingReset bool
}

type DriveState struct {
	Tick             uint64
	Paused           bool
	Speed            float64
	DebugLayers      scene.DebugLayers
	HistoryLen       int
	HistoryStartTick uint64
	HistoryEndTick   uint64
}

type AdvanceStats struct {
	Steps   int
	Clamped bool
	Alpha   float64
}

func NewDriver(game Game, log *slog.Logger) *Driver {
	cfg := game.Setup()
	if cfg.SimulationHz <= 0 {
		cfg.SimulationHz = defaultSimulationHz
	}
	if cfg.FPS <= 0 {
		cfg.FPS = defaultFPS
	}

	d := &Driver{
		game:     game,
		cfg:      cfg,
		log:      log.With("scope", "engine"),
		input:    newDriverInput(cfg.InputBindings),
		clock:    newClock(cfg.SimulationHz),
		controls: devControls{speed: 1},
	}
	d.uiTheme = cfg.UITheme
	if d.uiTheme == (ui.Theme{}) {
		d.uiTheme = ui.DefaultTheme()
	}
	if cfg.HistoryLen > 0 {
		d.recorder = newRecorder(cfg.HistoryLen)
	}
	d.doReset()
	return d
}

// ApplyInput receives raw events from the platform and applies them to driver input.
// Called by the platform before AdvanceTime or Step, within the same lock.
func (d *Driver) ApplyInput(events []input.Event) {
	d.input.Apply(events, d.tick)
}

// Control applies a ControlCommand to the internal state.
// Called by the platform before AdvanceTime or Step, within the same lock.
func (d *Driver) Control(cmd ControlCommand) {
	if d.controls.apply(cmd) {
		d.pendingReset = true
	}
}

// AdvanceTime advances using elapsed wall time. Platforms should use this entry
// point; deterministic replay and tests should prefer Step.
func (d *Driver) AdvanceTime(elapsed time.Duration) AdvanceStats {
	frame := d.controls.consume()

	if d.pendingReset {
		d.doReset()
		d.pendingReset = false
		// skips the reset tick (World has just been rebuilt)
		return AdvanceStats{Alpha: d.clock.Alpha()}
	}

	for range frame.backs {
		d.doBack()
	}
	for _, tick := range frame.backTicks {
		d.doBackToTick(tick)
	}
	for _, seconds := range frame.backSeconds {
		d.doBackSeconds(seconds)
	}

	if frame.paused && frame.steps == 0 {
		return AdvanceStats{Alpha: d.clock.Alpha()}
	}

	stepsToRun := frame.steps
	clamped := false
	if stepsToRun == 0 {
		plan := d.clock.Consume(elapsed, frame.speed)
		stepsToRun = plan.Steps
		clamped = plan.Clamped
	}

	d.runSteps(stepsToRun)

	return AdvanceStats{Steps: stepsToRun, Clamped: clamped, Alpha: d.clock.Alpha()}
}

// Step advances exactly one fixed simulation tick. It does not touch the clock
// accumulator, making it the deterministic entry point for tests and replay.
func (d *Driver) Step() AdvanceStats {
	frame := d.controls.consume()

	if d.pendingReset {
		d.doReset()
		d.pendingReset = false
		return AdvanceStats{Alpha: d.clock.Alpha()}
	}

	for range frame.backs {
		d.doBack()
	}
	for _, tick := range frame.backTicks {
		d.doBackToTick(tick)
	}
	for _, seconds := range frame.backSeconds {
		d.doBackSeconds(seconds)
	}

	d.runSteps(1)

	return AdvanceStats{Steps: 1, Alpha: d.clock.Alpha()}
}

func (d *Driver) runSteps(steps int) {
	if steps <= 0 {
		return
	}

	for range steps {
		d.stepOnce(d.clock.DT())
	}

	d.log.Debug("advance", "tick", d.tick, "checksum", d.world.Checksum())
}

func (d *Driver) stepOnce(dt float64) {
	if d.recorder != nil {
		d.recorder.Capture(d.captureSnapshot())
	}

	d.input.BeginTick()
	ctx := d.makeUpdateContext(d.newUpdateUI())
	d.game.Update(&ctx, dt)

	d.tick++
	d.input.EndTick(d.tick)
}

func (d *Driver) Commands() []render.Command {
	uiInst := d.newRenderUI()
	ctx := d.makeRenderContext(uiInst)
	gameCmds := d.game.Render(&ctx)
	cmds := append([]render.Command(nil), gameCmds...)
	cmds = append(cmds, d.debugCommands(gameCmds, ctx.Camera())...)
	cmds = append(cmds, uiInst.Commands()...)
	return cmds
}

func (d *Driver) State() DriveState {
	state := DriveState{
		Tick:        d.tick,
		Paused:      d.controls.paused,
		Speed:       d.controls.speed,
		DebugLayers: d.controls.debugLayers,
		HistoryLen:  d.HistoryLen(),
	}
	if d.recorder != nil {
		start, end, ok := d.recorder.Range()
		if ok {
			state.HistoryStartTick = start
			state.HistoryEndTick = end
		}
	}
	return state
}

func (d *Driver) Config() Config { return d.cfg }
func (d *Driver) Tick() uint64   { return d.tick }
func (d *Driver) HistoryLen() int {
	if d.recorder == nil {
		return 0
	}
	return d.recorder.Len()
}
func (d *Driver) CurrentSpeed() float64 { return d.controls.speed }

func (d *Driver) Frame() Frame {
	uiInst := d.newRenderUI()
	ctx := d.makeRenderContext(uiInst)
	gameCmds := d.game.Render(&ctx)
	cmds := append([]render.Command(nil), gameCmds...)
	cmds = append(cmds, d.debugCommands(gameCmds, ctx.Camera())...)
	cmds = append(cmds, uiInst.Commands()...)
	return Frame{
		Camera:   ctx.Camera(),
		Commands: cmds,
	}
}

func (d *Driver) debugCommands(gameCmds []render.Command, cam render.Camera) []render.Command {
	if d.controls.debugLayers == 0 {
		return nil
	}

	return debugRenderer{driver: d}.Commands(gameCmds, cam)
}

func (d *Driver) debugOverlayBounds() math2d.Rect {
	if d.cfg.Bounds.Width() > 0 && d.cfg.Bounds.Height() > 0 {
		return d.cfg.Bounds
	}

	return d.cfg.Camera.World
}

func (d *Driver) cameraForSurface() render.Camera {
	cam := d.cfg.Camera
	if d.viewport.Width() > 0 && d.viewport.Height() > 0 {
		cam.Viewport = d.viewport
	}
	return cam
}

// newRenderUI builds the draw face of the UI, used in Render/Init.
func (d *Driver) newRenderUI() *ui.UI {
	cam := d.cameraForSurface()
	pointer := d.input.Pointer()
	return ui.New(
		&d.uiState,
		pointer.Pos(),
		pointer.Inside(),
		cam,
		d.uiTheme,
		d.cfg.Camera.Viewport,
	)
}

// newUpdateUI builds the interaction face of the UI, used in Update.
func (d *Driver) newUpdateUI() *ui.UI {
	cam := d.cameraForSurface()
	return ui.NewUpdate(&d.uiState, d.input.PointerFrame(), cam, d.uiTheme, d.cfg.Camera.Viewport)
}

// SetViewport updates the surface size used for camera fitting and UI layout.
func (d *Driver) SetViewport(viewport math2d.Rect) {
	d.viewport = viewport
}

func (d *Driver) makeUpdateContext(uiInst *ui.UI) UpdateContext {
	return UpdateContext{
		World:      d.world,
		Input:      d.input.LastInput(),
		Pointer:    d.input.PointerFrame(),
		Log:        d.log.With("scope", "game"),
		Tick:       d.tick,
		UI:         uiInst,
		StepConfig: scene.StepConfig{Bounds: d.cfg.Bounds, Physics: d.cfg.Physics},
		camera:     d.cameraForSurface(),
	}
}

func (d *Driver) makeRenderContext(uiInst *ui.UI) RenderContext {
	return RenderContext{
		World:  d.world,
		UI:     uiInst,
		Time:   float64(d.tick) * d.clock.DT(),
		Dt:     d.clock.DT(),
		Log:    d.log.With("scope", "game"),
		Tick:   d.tick,
		camera: d.cameraForSurface(),
	}
}

// doReset rebuilds the World and calls Game.Init. Never called directly by the
// platform — it goes through Control(ControlReset) → pendingReset → AdvanceTime/Step.
func (d *Driver) doReset() {
	d.world = scene.NewWorld()
	d.tick = 0
	d.clock.Reset()
	d.uiState = ui.State{}
	d.input.Reset()
	if d.recorder != nil {
		d.recorder.Clear()
	}
	ctx := d.makeUpdateContext(d.newRenderUI())
	d.game.Init(&ctx)
	d.log.Info("reset")
}

// doBack reverts to a recorder snapshot.
func (d *Driver) doBack() {
	if d.recorder == nil {
		return
	}
	snap, ok := d.recorder.Back()
	if !ok {
		return
	}
	d.restoreSnapshot(snap)

	d.log.Debug("back", "tick", d.tick, "checksum", d.world.Checksum())
}

func (d *Driver) doBackToTick(target uint64) {
	if d.recorder == nil {
		return
	}
	snap, ok := d.recorder.BackToTick(d.tick, target)
	if !ok {
		return
	}
	d.restoreSnapshot(snap)

	d.log.Debug("backToTick", "tick", d.tick, "target", target)
}

func (d *Driver) doBackSeconds(seconds float64) {
	if seconds <= 0 {
		return
	}

	ticksBack := uint64(math.Ceil(seconds * float64(d.cfg.SimulationHz)))
	if ticksBack == 0 {
		return
	}

	target := uint64(0)
	if ticksBack < d.tick {
		target = d.tick - ticksBack
	}
	d.doBackToTick(target)
}

func (d *Driver) restoreSnapshot(snap driverSnapshot) {
	d.world = snap.world
	d.uiState = snap.uiState
	d.input.Restore(snap.input)
	d.tick = snap.tick

	if game, ok := d.game.(StatefulGame); ok {
		game.RestoreGameState(snap.gameState)
	}
}

func (d *Driver) captureSnapshot() driverSnapshot {
	return driverSnapshot{
		world:     d.world.Clone(),
		input:     d.input.Snapshot(),
		uiState:   d.uiState.Clone(),
		gameState: d.captureGameState(),
		tick:      d.tick,
	}
}

func (d *Driver) captureGameState() GameState {
	game, ok := d.game.(StatefulGame)
	if !ok {
		return nil
	}

	// StateSlot[T].GameState() already returns an independent value snapshot
	// (boxing a value type into the GameState interface copies it) — no
	// further cloning needed here, unlike world/input/uiState.
	return game.GameState()
}

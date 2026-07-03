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

	warnedManualStep bool

	// UI
	viewport math2d.Rect
	uiState  ui.State
	uiTheme  ui.Theme

	pendingReset bool

	// recording captures every event applied via ApplyInput, tagged with the
	// tick it landed on, while recordingEnabled is true. Unlike the internal
	// input.Buffer (a bounded window used for Back/BackToTick), this keeps
	// the full log so Replay can reproduce a run from tick 0.
	recordingEnabled bool
	recording        []input.Event

	// scrubLog is an always-on twin of recording, kept for ScrubToTick. It is
	// never bounded by HistoryLen (unlike the recorder's snapshot stack), so
	// it can reconstruct any tick in [0, maxTick] via Replay. maxTick is the
	// highest tick this driver has actually simulated; it only moves forward
	// during normal play, so scrubbing backward doesn't lower the ceiling —
	// scrubbing is non-destructive and works in both directions.
	//
	// The timeline forks (truncating the now-stale recorded future) at the
	// moment a simulation step runs from a tick earlier than maxTick — that
	// is when live state actually diverges from the log. Input alone never
	// forks: events arriving while rewound go to stagedEvents, are committed
	// to scrubLog at the fork, and are discarded if a scrub/back/reset
	// rewinds away first (their mapper effect is discarded by the same
	// restore, keeping log and live state consistent).
	scrubLog     []input.Event
	stagedEvents []input.Event
	maxTick      uint64

	// viewportLog is an always-on log of SetViewport calls, tagged with the
	// tick at which each occurred. Like scrubLog, it is never bounded and
	// forks at the same moment (a step from a rewound tick). The scrubber
	// threads it into replayWithViewport so UI hit-testing during replay uses
	// the viewport that was active when each pointer event was originally
	// recorded — without it, a viewport change between recording and replay
	// makes screen-pixel click coordinates map to the wrong reference-canvas
	// position, missing buttons and freezing UI-gated games (e.g. breakout's
	// Play click during scrub after a browser resize).
	viewportLog []viewportEntry

	// stagedViewport holds a viewport change that arrived while rewound
	// (tick < maxTick). It is committed to viewportLog at the fork (like
	// stagedEvents) or discarded by the next scrub/back/reset.
	stagedViewport    math2d.Rect
	hasStagedViewport bool
}

// viewportEntry records a SetViewport call at the tick it was applied.
type viewportEntry struct {
	tick     uint64
	viewport math2d.Rect
}

type DriveState struct {
	Tick             uint64
	Paused           bool
	Speed            float64
	DebugLayers      scene.DebugLayers
	HistoryLen       int
	HistoryStartTick uint64
	HistoryEndTick   uint64

	// MaxRecordedTick is the highest tick ScrubToTick can reach — the full
	// scrub range is [0, MaxRecordedTick], independent of HistoryLen.
	MaxRecordedTick uint64
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

	if d.recordingEnabled {
		for _, event := range events {
			event.Tick = d.tick
			d.recording = append(d.recording, event)
		}
	}

	for _, event := range events {
		event.Tick = d.tick
		if d.tick < d.maxTick {
			// Rewound: the log still holds the old future at this tick, so
			// fresh events can't be appended yet — stage them until a step
			// forks the timeline (or a scrub/back discards them).
			d.stagedEvents = append(d.stagedEvents, event)
		} else {
			d.scrubLog = append(d.scrubLog, event)
		}
	}
}

func truncateAtTick(events []input.Event, tick uint64) []input.Event {
	kept := events[:0]
	for _, e := range events {
		if e.Tick < tick {
			kept = append(kept, e)
		}
	}
	return kept
}

// truncateViewportLogAtTick drops viewport entries at or after the fork tick,
// matching truncateAtTick for input events.
func truncateViewportLogAtTick(log []viewportEntry, tick uint64) []viewportEntry {
	kept := log[:0]
	for _, ve := range log {
		if ve.tick < tick {
			kept = append(kept, ve)
		}
	}
	return kept
}

func appendViewportLogEntry(log []viewportEntry, tick uint64, viewport math2d.Rect) []viewportEntry {
	if viewport.Width() <= 0 || viewport.Height() <= 0 {
		return log
	}
	if len(log) > 0 && log[len(log)-1].viewport == viewport {
		return log
	}
	return append(log, viewportEntry{tick: tick, viewport: viewport})
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
	for _, tick := range frame.scrubTicks {
		d.doScrubToTick(tick)
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
	for _, tick := range frame.scrubTicks {
		d.doScrubToTick(tick)
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
	// A step from a rewound point is the moment live state diverges from the
	// recorded future — fork the timeline: drop the stale future and commit
	// the input staged since the rewind (all stamped with the current tick).
	if d.tick < d.maxTick {
		d.scrubLog = truncateAtTick(d.scrubLog, d.tick)
		d.scrubLog = append(d.scrubLog, d.stagedEvents...)
		d.stagedEvents = nil
		d.viewportLog = truncateViewportLogAtTick(d.viewportLog, d.tick)
		if d.hasStagedViewport {
			d.viewportLog = appendViewportLogEntry(d.viewportLog, d.tick, d.stagedViewport)
			d.stagedViewport = math2d.Rect{}
			d.hasStagedViewport = false
		} else {
			d.viewportLog = appendViewportLogEntry(d.viewportLog, d.tick, d.viewport)
		}
		d.maxTick = d.tick
	}

	if d.recorder != nil {
		d.recorder.Capture(d.captureSnapshot())
	}

	d.input.BeginTick()
	ctx := d.makeUpdateContext(d.newUpdateUI())
	stepsBefore := d.world.StepTick()
	d.game.Update(&ctx, dt)

	switch {
	case ctx.simulationHeld:
	case d.world.StepTick() != stepsBefore:
		d.warnManualStepOnce()
	default:
		stats := d.world.Step(dt, ctx.StepConfig)
		if postStepper, ok := d.game.(PostStepper); ok {
			postStepper.PostStep(&ctx, stats)
		}
	}

	d.tick++
	if d.tick > d.maxTick {
		d.maxTick = d.tick
	}
	d.input.EndTick(d.tick)
}

func (d *Driver) warnManualStepOnce() {
	if d.warnedManualStep {
		return
	}
	d.warnedManualStep = true
	d.log.Warn("game called World.Step inside Update; auto-step skipped; migrate to PostStep (AD-1)")
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
		Tick:            d.tick,
		Paused:          d.controls.paused,
		Speed:           d.controls.speed,
		DebugLayers:     d.controls.debugLayers,
		HistoryLen:      d.HistoryLen(),
		MaxRecordedTick: d.maxTick,
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

// Checksum returns a deterministic hash of the current world state. Two
// driver runs that reach the same tick via the same inputs always produce
// the same value — use it to verify a Replay reproduced a recording exactly.
func (d *Driver) Checksum() uint64 { return d.world.Checksum() }

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
// The change is recorded in viewportLog (tagged with the current tick) so the
// scrubber can replay it at the correct tick — pointer event coordinates are
// in screen pixels and only map to the right UI element with the viewport that
// was active when they were recorded. When rewound (tick < maxTick), the
// change is staged until a fork commits it, matching the staging of input
// events.
func (d *Driver) SetViewport(viewport math2d.Rect) {
	if d.viewport == viewport {
		return
	}

	d.viewport = viewport
	if d.tick < d.maxTick {
		if d.hasStagedViewport && d.stagedViewport == viewport {
			return
		}
		d.stagedViewport = viewport
		d.hasStagedViewport = true
	} else {
		d.viewportLog = appendViewportLogEntry(d.viewportLog, d.tick, viewport)
	}
}

func (d *Driver) makeUpdateContext(uiInst *ui.UI) UpdateContext {
	return UpdateContext{
		World:      d.world,
		Input:      d.input.LastInput(),
		Pointer:    d.input.PointerFrame(),
		Log:        d.log.With("scope", "game"),
		Tick:       d.tick,
		UI:         uiInst,
		StepConfig: scene.StepConfig{Bounds: d.cfg.Bounds, Bounce: d.cfg.Bounce, Physics: d.cfg.Physics},
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
	d.maxTick = 0
	d.scrubLog = nil
	d.stagedEvents = nil
	d.viewportLog = nil
	d.stagedViewport = math2d.Rect{}
	d.hasStagedViewport = false
	if d.recordingEnabled {
		// Ticks restart at 0: keeping the pre-reset events would collide
		// with the new run's ticks and Replay would apply both. Restart the
		// capture so StopRecording returns a reproducible post-reset log.
		d.recording = nil
		d.log.Info("reset restarted the active input recording")
	}
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

// doScrubToTick moves the driver to target by deterministically replaying
// the full scrub log from tick 0, then adopting the replayed driver's
// world/input/recorder/tick. It's a no-op outside [0, maxTick]. Unlike
// doBack/doBackToTick, it never discards scrubLog, so scrubbing backward
// and then forward again always lands on the exact original state.
func (d *Driver) doScrubToTick(target uint64) {
	if target > d.maxTick {
		return
	}

	// Thread the live viewport AND the viewport change log so
	// viewport-dependent Update logic (UI hit-testing above all) reconstructs
	// identically — the viewport at each tick during replay matches what was
	// active when the pointer events were originally recorded.
	replayed := replayWithViewport(d.game, d.log, Recording{Events: d.scrubLog, Ticks: target}, d.viewport, d.viewportLog)

	d.world = replayed.world
	d.uiState = replayed.uiState
	d.input = replayed.input
	d.recorder = replayed.recorder
	d.tick = replayed.tick
	// The replayed mapper carries none of the input staged since the last
	// rewind — discard the staged events and staged viewport too, keeping
	// log and state aligned.
	d.stagedEvents = nil
	d.hasStagedViewport = false

	// The replay suppressed snapshot capture (see replayWithViewport) to
	// avoid cloning the world on every replay tick. Rebuild the recorder with
	// a single snapshot at the target tick so Back still works after a scrub.
	if d.cfg.HistoryLen > 0 {
		d.recorder = newRecorder(d.cfg.HistoryLen)
		d.recorder.Capture(d.captureSnapshot())
	}

	d.log.Debug("scrubToTick", "tick", d.tick, "target", target)
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
	// Restoring replaces the mapper wholesale, dropping the effect of any
	// staged input — drop the staged events and staged viewport with it
	// (see stepOnce's fork).
	d.stagedEvents = nil
	d.hasStagedViewport = false

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

package runtime

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

const defaultSimulationHz = 60
const defaultFPS = 60
const defaultKeyframeInterval = 30

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
	rng      Rand
	tick     uint64
	log      *slog.Logger

	warnedManualStep bool

	// UI
	viewport   math2d.Rect
	uiState    ui.State
	uiTheme    ui.Theme
	renderTime float64
	renderDt   float64
	animStore  ui.AnimStore

	pendingReset bool

	// Recording keeps the full input log while explicit recording is enabled.
	recordingEnabled bool
	recording        []input.Event
	recordingAnchors []Anchor
	// recordingInitialViewport is the surface size captured at StartRecording
	// (tick 0) so StopRecording can persist it; Replay must restore it or
	// viewport-dependent logic (UI hit-tests) diverges from the live run.
	recordingInitialViewport math2d.Rect

	// scrubLog is the unbounded input log used by ScrubToTick.
	// A step from an older tick forks the timeline and drops stale future input.
	scrubLog     []input.Event
	stagedEvents []input.Event
	maxTick      uint64

	// viewportLog lets replay map screen-space pointer events with the viewport
	// that was active when each event was recorded.
	viewportLog []viewportEntry

	// stagedViewport follows the same fork/discard rules as stagedEvents.
	stagedViewport    math2d.Rect
	hasStagedViewport bool

	determinismViolated bool

	// Audio

	audio     driverAudio
	audioPort AudioPort
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

	// DeterminismViolated stays true after a seek reconstructs a keyframed
	// tick with a checksum different from the recorded keyframe checksum.
	DeterminismViolated bool
}

type AdvanceStats struct {
	Steps   int
	Clamped bool
	Alpha   float64
}

func NewDriver(game Game, log *slog.Logger) *Driver {
	return newDriver(game, log, nil)
}

func newDriver(game Game, log *slog.Logger, seedOverride *uint64) *Driver {
	cfg := game.Setup()
	if seedOverride != nil {
		cfg.Seed = *seedOverride
	}
	if cfg.SimulationHz <= 0 {
		cfg.SimulationHz = defaultSimulationHz
	}
	if cfg.FPS <= 0 {
		cfg.FPS = defaultFPS
	}
	if cfg.KeyframeInterval <= 0 {
		cfg.KeyframeInterval = defaultKeyframeInterval
	}

	d := &Driver{
		game:     game,
		cfg:      cfg,
		log:      log.With("scope", "engine"),
		input:    newDriverInput(cfg.InputBindings),
		clock:    newClock(cfg.SimulationHz),
		controls: devControls{speed: 1},
	}
	d.uiTheme = ui.WithDefaults(cfg.UITheme)
	if cfg.HistoryLen > 0 {
		d.recorder = newRecorder(cfg.HistoryLen, cfg.KeyframeInterval)
	}
	d.rng = NewRand(cfg.Seed)
	d.audioPort = AudioPort{audio: &d.audio}
	d.doReset()
	return d
}

// ApplyInput applies platform events before AdvanceTime or Step.
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

// Control applies a platform control command before AdvanceTime or Step.
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
		// Skip the reset tick; World has just been rebuilt.
		return AdvanceStats{Alpha: d.clock.Alpha()}
	}

	if target, ok := d.coalesceSeeks(frame.seeks); ok {
		d.seekTo(target)
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

	if target, ok := d.coalesceSeeks(frame.seeks); ok {
		d.seekTo(target)
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

	// Checksum is O(entities); slog evaluates arguments even when the debug
	// handler is disabled, so gate the call explicitly.
	if d.log.Enabled(context.Background(), slog.LevelDebug) {
		d.log.Debug("advance", "tick", d.tick, "checksum", d.world.Checksum())
	}
}

func (d *Driver) stepOnce(dt float64) {
	// Stepping from a rewound tick forks the timeline.
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
		if d.recorder != nil {
			d.recorder.DropAfter(d.tick)
		}
	}

	if d.recorder != nil && d.recorder.WantsCapture(d.tick) {
		d.recorder.Capture(d.captureSnapshot())
	}
	if d.recordingEnabled && d.tick%uint64(d.cfg.KeyframeInterval) == 0 {
		d.recordingAnchors = append(d.recordingAnchors, Anchor{Tick: d.tick, Checksum: d.world.Checksum()})
	}

	// Capture render pose before the simulation step so the next frame
	// has a valid previous state for interpolation.
	d.world.SyncRenderState()

	d.stepCore(dt)

	if d.tick > d.maxTick {
		d.maxTick = d.tick
	}
}

func (d *Driver) stepCore(dt float64) {
	d.audio.beginTick(d.tick)
	d.input.BeginTick()
	uiInst := d.newUpdateUI()
	ctx := d.makeUpdateContext(uiInst)
	stepsBefore := d.world.StepTick()
	d.game.Update(&ctx, dt)
	uiInst.EndFrame()

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
	d.input.EndTick(d.tick)
}

func (d *Driver) warnManualStepOnce() {
	if d.warnedManualStep {
		return
	}
	d.warnedManualStep = true
	d.log.Warn("game called World.Step inside Update; auto-step skipped; migrate to PostStep (AD-1)")
}

func (d *Driver) State() DriveState {
	state := DriveState{
		Tick:                d.tick,
		Paused:              d.controls.paused,
		Speed:               d.controls.speed,
		DebugLayers:         d.controls.debugLayers,
		HistoryLen:          d.historyLen(),
		MaxRecordedTick:     d.maxTick,
		DeterminismViolated: d.determinismViolated,
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
func (d *Driver) historyLen() int {
	if d.recorder == nil {
		return 0
	}
	return d.recorder.Len()
}

// Checksum returns a deterministic hash of the current world state. Two
// driver runs that reach the same tick via the same inputs always produce
// the same value — use it to verify a Replay reproduced a recording exactly.
func (d *Driver) Checksum() uint64 { return d.world.Checksum() }

func (d *Driver) Frame(elapsed time.Duration) Frame {
	d.renderDt = elapsed.Seconds()
	d.renderTime += d.renderDt
	d.animStore.AdvanceFrame()
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

func (d *Driver) ConsumeAudio() audio.Output {
	return d.audio.consume()
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
		&d.animStore,
		d.renderDt,
		d.renderTime,
	)
}

// newUpdateUI builds the interaction face of the UI, used in Update.
func (d *Driver) newUpdateUI() *ui.UI {
	cam := d.cameraForSurface()
	return ui.NewUpdateWithInput(&d.uiState, d.input.PointerFrame(), d.input.LastInput(), cam, d.uiTheme, d.cfg.Camera.Viewport)
}

// SetViewport updates the surface size used for camera fitting and UI layout.
// Rewound viewport changes are staged until the next timeline fork.
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

func (d *Driver) makeRenderContext(uiInst *ui.UI) RenderContext {
	alpha := 0.0
	if d.cfg.RenderInterpolation {
		alpha = d.clock.Alpha()
	}
	return RenderContext{
		World:  d.world,
		Assets: d.cfg.Assets,
		UI:     uiInst,
		Time:   d.renderTime,
		Dt:     d.renderDt,
		Log:    d.log.With("scope", "game"),
		Tick:   d.tick,
		Alpha:  alpha,
		camera: d.cameraForSurface(),
	}
}

// doReset rebuilds the World and calls Game.Init.
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
		d.recordingAnchors = nil
		// The viewport survives a reset (doReset never clears d.viewport), so
		// the post-reset recording's tick-0 viewport is the current surface.
		d.recordingInitialViewport = d.viewport
		d.log.Info("reset restarted the active input recording")
	}
	d.clock.Reset()
	d.uiState = ui.State{}
	d.animStore = ui.AnimStore{}
	d.renderTime = 0
	d.renderDt = 0
	d.input.Reset()
	d.determinismViolated = false
	if d.recorder != nil {
		d.recorder.Clear()
	}
	ctx := d.makeUpdateContext(d.newRenderUI())
	d.rng = NewRand(d.cfg.Seed)
	d.audio.reset()
	d.game.Init(&ctx)
	d.log.Info("reset")
}

func (d *Driver) restoreSnapshot(snap driverSnapshot) {
	d.world = snap.world.Clone()
	d.uiState = snap.uiState.Clone()
	d.input.Restore(cloneDriverInputSnapshot(snap.input))
	d.rng = snap.rng
	d.viewport = snap.viewport
	d.tick = snap.tick
	d.audio.state = snap.audioState.Clone()
	d.audio.pending = nil
	// Restoring replaces the mapper wholesale, dropping the effect of any
	// staged input — drop the staged events and staged viewport with it
	// (see stepOnce's fork).
	d.stagedEvents = nil
	d.hasStagedViewport = false

	if game, ok := d.game.(StatefulGame); ok {
		game.RestoreGameState(snap.gameState)
	}
}

func cloneDriverInputSnapshot(snapshot driverInputSnapshot) driverInputSnapshot {
	return driverInputSnapshot{
		mapper:    snapshot.mapper.Clone(),
		lastInput: snapshot.lastInput.Clone(),
		pointer:   snapshot.pointer.Clone(),
	}
}

func (d *Driver) captureSnapshot() driverSnapshot {
	return driverSnapshot{
		world:      d.world.Clone(),
		input:      d.input.Snapshot(),
		uiState:    d.uiState.Clone(),
		gameState:  d.captureGameState(),
		rng:        d.rng,
		viewport:   d.viewport,
		tick:       d.tick,
		audioState: d.audio.state,
		checksum:   d.world.Checksum(),
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

func (d *Driver) coalesceSeeks(seeks []seekRequest) (target uint64, ok bool) {
	target = d.tick
	for _, seek := range seeks {
		switch seek.kind {
		case seekBack:
			if target > 0 {
				target--
			}
		case seekToTick:
			if seek.tick < target {
				target = seek.tick
			}
		case seekBackSeconds:
			if seek.seconds <= 0 {
				continue
			}
			back := uint64(math.Ceil(seek.seconds * float64(d.cfg.SimulationHz)))
			if back > target {
				target = 0
			} else {
				target -= back
			}
		case seekScrub:
			if seek.tick <= d.maxTick {
				target = seek.tick
			}
		}
	}
	return target, target != d.tick
}

// seekTo moves the driver to target ∈ [0, maxTick]. It never truncates scrubLog
// or keyframes — the fork remains exclusive to stepOnce.
func (d *Driver) seekTo(target uint64) {
	if target > d.maxTick || target == d.tick {
		return
	}

	surfaceViewport := d.viewport

	// Fast-forward can be incremental only when the current state has not been
	// tainted by staged input/viewport changes that are waiting for a fork.
	if target > d.tick && len(d.stagedEvents) == 0 && !d.hasStagedViewport {
		d.viewport = d.viewportAtTick(d.tick, d.viewport)
		d.resimulate(target)
		d.viewport = surfaceViewport
		return
	}

	restored := false
	if d.recorder != nil && d.canRestoreKeyframes() {
		if snap, ok := d.recorder.KeyframeAtOrBefore(target); ok {
			d.restoreSnapshot(snap)
			restored = true
		}
	}
	if !restored {
		d.resetForResimulation()
	}
	d.viewport = d.viewportAtTick(d.tick, d.viewport)
	if !restored && target == 0 {
		d.applyLoggedInputsAtCurrentTick()
	}
	d.resimulate(target)
	d.audio.pending = d.audio.pending[:0]
	d.viewport = surfaceViewport

	if d.log.Enabled(context.Background(), slog.LevelDebug) {
		d.log.Debug("seek", "tick", d.tick, "target", target, "checksum", d.world.Checksum())
	}
}

func (d *Driver) resetForResimulation() {
	d.world = scene.NewWorld()
	d.tick = 0
	d.uiState = ui.State{}
	d.animStore = ui.AnimStore{}
	d.input.Reset()
	d.stagedEvents = nil
	d.stagedViewport = math2d.Rect{}
	d.hasStagedViewport = false
	d.rng = NewRand(d.cfg.Seed)
	d.audio.reset()
	if game, ok := d.game.(StatefulGame); ok {
		game.RestoreGameState(nil)
	}
	ctx := d.makeUpdateContext(d.newRenderUI())
	d.game.Init(&ctx)
}

func (d *Driver) canRestoreKeyframes() bool {
	_, ok := d.game.(StatefulGame)
	return ok
}

func (d *Driver) resimulate(target uint64) {
	cursor := newResimulationCursor(d.scrubLog, d.viewportLog, d.tick)
	dt := d.clock.DT()

	for d.tick < target {
		for _, viewport := range cursor.viewportsAt(d.tick) {
			d.viewport = viewport
		}
		d.input.Apply(cursor.eventsAt(d.tick), d.tick)
		d.stepCore(dt)
		d.verifyKeyframe(d.tick)
	}
	d.stagedEvents = nil
	d.stagedViewport = math2d.Rect{}
	d.hasStagedViewport = false
}

func (d *Driver) applyLoggedInputsAtCurrentTick() {
	cursor := newResimulationCursor(d.scrubLog, d.viewportLog, d.tick)
	for _, viewport := range cursor.viewportsAt(d.tick) {
		d.viewport = viewport
	}
	d.input.Apply(cursor.eventsAt(d.tick), d.tick)
}

func (d *Driver) verifyKeyframe(tick uint64) {
	if d.recorder == nil || !d.canRestoreKeyframes() {
		return
	}
	keyframe, ok := d.recorder.KeyframeAt(tick)
	if !ok {
		return
	}
	got := d.world.Checksum()
	if got == keyframe.checksum {
		return
	}
	d.determinismViolated = true
	d.log.Error("resimulation diverged from keyframe", "tick", tick, "got", got, "want", keyframe.checksum)
}

func (d *Driver) viewportAtTick(tick uint64, fallback math2d.Rect) math2d.Rect {
	viewport := fallback
	for _, entry := range d.viewportLog {
		if entry.tick > tick {
			break
		}
		viewport = entry.viewport
	}
	return viewport
}

type resimulationCursor struct {
	events        []input.Event
	eventIndex    int
	viewportLog   []viewportEntry
	viewportIndex int
}

func newResimulationCursor(events []input.Event, viewportLog []viewportEntry, tick uint64) resimulationCursor {
	cursor := resimulationCursor{events: events, viewportLog: viewportLog}
	for cursor.eventIndex < len(cursor.events) && cursor.events[cursor.eventIndex].Tick < tick {
		cursor.eventIndex++
	}
	for cursor.viewportIndex < len(cursor.viewportLog) && cursor.viewportLog[cursor.viewportIndex].tick < tick {
		cursor.viewportIndex++
	}
	return cursor
}

func (c *resimulationCursor) eventsAt(tick uint64) []input.Event {
	for c.eventIndex < len(c.events) && c.events[c.eventIndex].Tick < tick {
		c.eventIndex++
	}
	start := c.eventIndex
	for c.eventIndex < len(c.events) && c.events[c.eventIndex].Tick == tick {
		c.eventIndex++
	}
	return c.events[start:c.eventIndex]
}

func (c *resimulationCursor) viewportsAt(tick uint64) []math2d.Rect {
	for c.viewportIndex < len(c.viewportLog) && c.viewportLog[c.viewportIndex].tick < tick {
		c.viewportIndex++
	}
	start := c.viewportIndex
	for c.viewportIndex < len(c.viewportLog) && c.viewportLog[c.viewportIndex].tick == tick {
		c.viewportIndex++
	}
	entries := c.viewportLog[start:c.viewportIndex]
	viewports := make([]math2d.Rect, 0, len(entries))
	for _, entry := range entries {
		viewports = append(viewports, entry.viewport)
	}
	return viewports
}

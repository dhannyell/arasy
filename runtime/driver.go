package runtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhannyell/arasy/audio"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/enginesplash"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

const defaultSimulationHz = 60
const defaultFPS = 60
const defaultKeyframeInterval = 30

const keyframeRestoreDisabledWarning = "keyframe restore disabled: game does not implement StatefulGame; every backward seek replays from tick 0; embed runtime.StateSlot to opt in"

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
	// gamepadConnected is a host-owned supported-gamepad availability hint.
	// It deliberately stays outside deterministic snapshots and recordings.
	gamepadConnected bool

	// debugChecksum memoizes World.Checksum for the runtime-stats overlay.
	// The checksum walks every component store, and the overlay asks for it
	// once per rendered frame while it can only change once per tick.
	debugChecksum      uint64
	debugChecksumTick  uint64
	debugChecksumValid bool

	splash *splashPlayer

	pendingReset bool

	// Recording keeps the full input log while explicit recording is enabled.
	recordingEnabled bool
	recording        []input.Event
	// recordingStaged mirrors timeline.staged for the recording log: events
	// applied while rewound (d.tick < d.timeline.maxTick) are held here
	// instead of d.recording, so a later scrub that discards the staged
	// timeline edits (never applied by the live run) also discards their
	// recorded entry. Without this, StopRecording/Replay could reproduce an
	// event the live run never actually stepped through.
	recordingStaged  []input.Event
	recordingAnchors []Anchor
	// recordingInitialViewport is the surface size captured at StartRecording
	// (tick 0) so StopRecording can persist it; Replay must restore it or
	// viewport-dependent logic (UI hit-tests) diverges from the live run.
	recordingInitialViewport math2d.Rect

	// timeline owns the unbounded scrub journal, staged fork edits, viewport
	// history, and maximum reachable tick.
	timeline timeline

	determinismViolated bool

	// Audio

	audio     driverAudio
	audioPort AudioPort
	texture   driverTexture
	batches   driverBatches
	batchPort BatchPort
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

	// ReplayMode is true while the driver advances by consuming the
	// recorded journal instead of stepping live.
	ReplayMode bool
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
	if cfg.FPS == 0 {
		cfg.FPS = defaultFPS
	}
	if cfg.KeyframeInterval <= 0 {
		cfg.KeyframeInterval = defaultKeyframeInterval
	}
	cfg.GamepadDeadzone = input.EffectiveGamepadDeadzone(cfg.GamepadDeadzone)
	if log == nil {
		log = slog.Default()
	}
	if !cfg.DisableDefaultSplash {
		// Load default engine splash assets
		cfg.Assets = enginesplash.LoadAssets(cfg.Assets)
	}

	d := &Driver{
		game:     game,
		cfg:      cfg,
		input:    newDriverInputWithDeadzone(cfg.InputBindings, cfg.GamepadDeadzone),
		clock:    newClock(cfg.SimulationHz),
		controls: devControls{speed: 1},
		log:      log.With("scope", "engine"),
	}

	d.uiTheme = ui.WithDefaults(cfg.UITheme)
	if cfg.HistoryLen > 0 {
		d.recorder = newRecorderWithArchiveBudget(cfg.HistoryLen, cfg.KeyframeInterval, cfg.KeyframeArchiveBudget)
		if !d.canRestoreKeyframes() {
			d.log.Warn(keyframeRestoreDisabledWarning)
		}
	}
	d.rng = NewRand(cfg.Seed)
	d.audioPort = AudioPort{audio: &d.audio}
	d.texture = driverTexture{}
	d.batchPort = BatchPort{batches: &d.batches}
	d.splash = &splashPlayer{segments: buildSplashPlaylist(cfg)}
	d.doReset()
	return d
}

// ApplyInput applies platform events before AdvanceTime or Step.
func (d *Driver) ApplyInput(events []input.Event) {
	d.input.Apply(events, d.tick)

	if d.recordingEnabled {
		rewound := d.tick < d.timeline.maxTick
		for _, event := range events {
			event.Tick = d.tick
			if rewound {
				// Staged like the timeline: only committed to d.recording if
				// stepOnce later forks from this tick; discarded otherwise.
				d.recordingStaged = append(d.recordingStaged, event)
			} else {
				d.recording = append(d.recording, event)
			}
		}
	}

	d.timeline.record(d.tick, events)
}

// Control applies a platform control command before AdvanceTime or Step.
func (d *Driver) Control(cmd ControlCommand) {
	watching := d.controls.replayMode
	if d.controls.apply(cmd) {
		d.pendingReset = true
	}
	// Taking control ends the recording's presses. Until this point the
	// journal owned them and would have released them at the tick it recorded
	// the release; from here the driver never reaches that tick, so a key
	// held at the moment control was taken would stay down forever. A reset
	// needs none of this — it rebuilds the input state outright.
	if watching && !d.controls.replayMode && !d.pendingReset {
		d.releaseWatchedInput()
	}
}

// releaseWatchedInput ends every press the recording left held.
//
// The releases are applied as ordinary input rather than written straight
// into the input state, so they land in the journal alongside everything
// else. A later scrub back across this tick then reproduces the release; had
// the state been edited directly, resimulation would rebuild a world where
// the key was never let go and the checksum would diverge from the live run.
func (d *Driver) releaseWatchedInput() {
	events := d.input.releaseEvents()
	if len(events) == 0 {
		return
	}
	d.log.Debug("released input held by the recording", "tick", d.tick, "count", len(events))
	d.ApplyInput(events)
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

	if frame.replayMode {
		d.playbackSteps(stepsToRun)
	} else {
		d.runSteps(stepsToRun)
	}

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

	if frame.replayMode {
		d.playbackSteps(1)
	} else {
		d.runSteps(1)
	}

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
	if d.tick < d.timeline.maxTick {
		d.timeline.fork(d.tick, d.viewport)
		if d.recordingEnabled {
			d.recording = truncateAtTick(d.recording, d.tick)
			d.recording = append(d.recording, d.recordingStaged...)
			d.recordingStaged = nil
		}
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

	d.timeline.advanceTo(d.tick)
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

// The development-only accessors the dev panel's inspector needs live in
// devaccess_dev.go, behind the arr_dev build tag.

func (d *Driver) State() DriveState {
	state := DriveState{
		Tick:                d.tick,
		Paused:              d.controls.paused,
		Speed:               d.controls.speed,
		DebugLayers:         d.controls.debugLayers,
		HistoryLen:          d.historyLen(),
		MaxRecordedTick:     d.timeline.maxTick,
		DeterminismViolated: d.determinismViolated,
		ReplayMode:          d.controls.replayMode,
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

// SetGamepadConnected updates the render-only supported-gamepad availability
// hint. Interactive hosts should call it when polling; it has no simulation
// effect.
func (d *Driver) SetGamepadConnected(connected bool) {
	// Unplugging is the one event that invalidates the debug mirror of the
	// pad: no release events arrive for a controller that is gone, so without
	// this a button held at the moment it was pulled would read as held
	// forever.
	if d.gamepadConnected && !connected {
		d.input.gamepad.reset()
	}
	d.gamepadConnected = connected
}

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

func (d *Driver) ConsumeTextureUpdates() []TextureUpdate { return d.texture.consume() }

// ConsumeFrameUpdates drains every pending render-only resource publication
// (dynamic textures and retained sprite/text batches) in one call. Texture
// updates are listed before batches so renderers can apply them first.
func (d *Driver) ConsumeFrameUpdates() FrameUpdates {
	sprites, text := d.batches.consume()
	return FrameUpdates{
		Textures:      d.texture.consume(),
		SpriteBatches: sprites,
		TextBatches:   text,
	}
}

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
	return d.audio.consume(d.tick, d.clock.Alpha())
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
	return ui.NewUpdateWithKeys(&d.uiState, d.input.PointerFrame(), d.input.KeyFrame(), d.input.LastInput(), cam, d.uiTheme, d.cfg.Camera.Viewport)
}

// SetViewport updates the surface size used for camera fitting and UI layout.
// Rewound viewport changes are staged until the next timeline fork.
func (d *Driver) SetViewport(viewport math2d.Rect) {
	if d.viewport == viewport {
		return
	}

	d.viewport = viewport
	d.timeline.recordViewport(d.tick, viewport)
}

func (d *Driver) makeRenderContext(uiInst *ui.UI) RenderContext {
	alpha := 0.0
	if d.cfg.RenderInterpolation {
		alpha = d.clock.Alpha()
	}
	return RenderContext{
		World:              d.world.View(),
		Assets:             d.cfg.Assets,
		UI:                 uiInst.Draw(),
		Time:               d.renderTime,
		Dt:                 d.renderDt,
		Log:                d.log.With("scope", "game"),
		Tick:               d.tick,
		Alpha:              alpha,
		GamepadConnected:   d.gamepadConnected,
		UpdateTextureField: &TexturePort{texture: &d.texture},
		Batches:            &d.batchPort,
		camera:             d.cameraForSurface(),
	}
}

// doReset rebuilds the World and calls Game.Init.
func (d *Driver) doReset() {
	d.world = scene.NewWorld()
	d.tick = 0
	d.timeline.reset()
	// A reset is the one way the world changes without the tick changing.
	d.debugChecksumValid = false
	if d.recordingEnabled {
		// Ticks restart at 0: keeping the pre-reset events would collide
		// with the new run's ticks and Replay would apply both. Restart the
		// capture so StopRecording returns a reproducible post-reset log.
		d.recording = nil
		d.recordingStaged = nil
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
	d.texture.reset()
	d.batches.reset()
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
	d.timeline.discardStaged()
	d.recordingStaged = nil

	if game, ok := d.game.(StatefulGame); ok {
		game.RestoreGameState(snap.gameState)
	}
}

// cloneDriverInputSnapshot deep-copies a snapshot so restoring it twice does
// not hand out the same mapper. Every field is carried: a restore that
// dropped one would silently reset that part of the input state, which reads
// as a key that was held before the rewind and is not held after it.
func cloneDriverInputSnapshot(snapshot driverInputSnapshot) driverInputSnapshot {
	return driverInputSnapshot{
		mapper:    snapshot.mapper.Clone(),
		lastInput: snapshot.lastInput.Clone(),
		pointer:   snapshot.pointer.Clone(),
		keyboard:  snapshot.keyboard.Clone(),
		held:      append([]heldInput(nil), snapshot.held...),
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
		audioState: d.audio.state.Clone(),
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

func (d *Driver) canRestoreKeyframes() bool {
	_, ok := d.game.(StatefulGame)
	return ok
}

package runtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/asset"
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
		// Registered directly (no manifest entry): WithTexture/WithEffect
		// only ever append, so this never reassigns or reorders the game's
		// own declared texture/effect IDs. Must happen before Driver.Config()
		// is read downstream (renderer construction), so the augmented
		// manifest carries them from the start. These stay resident in the
		// manifest for the game's whole lifetime rather than being
		// registered into a temporary manifest and discarded after the
		// splash: backends resolve texture/effect IDs against the single
		// manifest a renderer is constructed with, so there is no point at
		// which the splash's assets could be un-registered without
		// reconstructing the renderer — and every one of them is small
		// enough (two generated 32x32 sprites, a handful of EffectDecl
		// values) that leaving them registered costs nothing.
		cfg.Assets, _ = cfg.Assets.WithTexture(enginesplash.EngineMoteTextureName, enginesplash.EngineMoteTexture())
		cfg.Assets, _ = cfg.Assets.WithTexture(enginesplash.EngineEmberTextureName, enginesplash.EngineEmberTexture())
		cfg.Assets, _ = cfg.Assets.WithEffect(enginesplash.EngineSparkEffectName, enginesplash.EngineSparkEffect())
		cfg.Assets, _ = cfg.Assets.WithEffect(enginesplash.EngineStarfieldEffectName, enginesplash.EngineStarfieldEffect())
		cruxEffects := enginesplash.EngineCruxEffects()
		for _, name := range enginesplash.EngineCruxNames() {
			effectName := enginesplash.EngineCruxEffectName(name)
			cfg.Assets, _ = cfg.Assets.WithEffect(effectName, cruxEffects[effectName])
		}
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

// buildSplashPlaylist assembles the boot splash playlist: the built-in
// engine brand segment (unless disabled), followed by the manifest-declared
// segments in order.
func buildSplashPlaylist(cfg Config) []SplashSegment {
	var segments []SplashSegment
	if !cfg.DisableDefaultSplash {
		segments = append(segments, SplashSegment{
			Content: func(dst []render.Command, timeline anim.Timeline, viewport math2d.Rect) []render.Command {
				return enginesplash.AppendArasySplash(dst, timeline, viewport, enginesplash.ArasySplashResources{})
			},
			MinTime:    enginesplash.EngineSplashMinTime,
			Fade:       enginesplash.EngineSplashFade,
			Skippable:  false,
			Background: enginesplash.EngineSplashBackground,
			Particles:  engineSplashParticles(cfg.Assets),
		})
	}
	for _, decl := range cfg.Assets.Splash() {
		segments = append(segments, SplashSegment{
			Texture:   cfg.Assets.Texture(decl.Texture),
			CellW:     decl.CellW,
			CellH:     decl.CellH,
			Columns:   decl.Columns,
			Frames:    decl.Frames,
			Rate:      decl.Rate,
			MinTime:   decl.MinTime,
			Fade:      decl.Fade,
			Skippable: decl.Skippable,
			Particles: manifestSplashParticles(cfg.Assets, decl),
		})
	}
	return segments
}

// engineSkyLayer is the starfield's and Crux's LayerValue: below
// engineSplashContentLayer (Content's default, unset BaseCommand.LayerValue
// == 0), so the logo's own opaque shapes occlude the sky behind them the
// way they would a real one, instead of the field needing a keep-clear zone
// carved out around the ring/wordmark. This only works because the compiled
// splash's content paints no full-canvas backdrop rect of its own at that
// layer — engineSplashBackgroundLayer sits below engineSkyLayer so the sky
// is never itself occluded by the segment's own backdrop rect (see
// splashFrame in runtime/splash.go, which paints Background at this same
// layer).
const (
	engineSplashBackgroundLayer = -2
	engineSkyLayer              = -1
)

// engineSplashParticles combines every particle system the engine's default
// splash draws: the ring spark (anchored to the compiled animation's own
// ring, tracking its letterbox scale, drawn in FRONT of the content since it
// reads as erupting off the ring), the uniform ambient starfield, and the
// five Crux (Southern Cross) stars (both drawn BEHIND the content) — each
// its own emitter, resolved once here against the effects newDriver
// registered via WithEffect, so a missing effect (DisableDefaultSplash games
// that still set Content some other way) simply draws nothing rather than
// panicking.
func engineSplashParticles(assets asset.Manifest) func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	sparkEffect := assets.Effect(enginesplash.EngineSparkEffectName)
	starfieldEffect := assets.Effect(enginesplash.EngineStarfieldEffectName)

	type namedEffect struct {
		name   string
		effect render.ParticleEffectID
	}
	cruxStars := make([]namedEffect, 0, len(enginesplash.EngineCruxNames()))
	for _, name := range enginesplash.EngineCruxNames() {
		cruxStars = append(cruxStars, namedEffect{name, assets.Effect(enginesplash.EngineCruxEffectName(name))})
	}

	return func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
		commands := make([]render.DrawParticles, 0, 2+len(cruxStars))

		if sparkEffect != 0 {
			if intensity := enginesplash.EngineSparkIntensity(elapsed) * alpha; intensity > 0 {
				center, radius := enginesplash.EngineSplashRingAnchor(viewport)
				scale := radius / enginesplash.RingRadius
				commands = append(commands, render.DrawParticles{
					BaseCommand: render.BaseCommand{LayerValue: 2},
					Effect:      sparkEffect,
					Emitter:     1,
					Pos:         center,
					EmitScale:   math2d.Vec2{X: scale, Y: scale},
					Intensity:   intensity,
					Time:        elapsed,
					Dt:          dt,
					Burst:       1,
					Seed:        42,
				})
			}
		}

		if alpha > 0 {
			contentScale := enginesplash.EngineSplashScale(viewport)
			cruxScale := math2d.Vec2{X: contentScale, Y: contentScale}
			skyScale := enginesplash.EngineStarfieldScale(viewport)
			// EmitterSet only rebuilds an effect's live particles when its
			// Seed changes (particle/emitterset.go); a constant Seed here
			// would leave every already-spawned star exactly where it was
			// emitted (starfield/Crux have no velocity) even after a resize
			// moves their anchor and rescales their spread, so most of the
			// sky would stay stuck at the previous viewport's layout for the
			// rest of that star's multi-second lifetime. Folding the
			// viewport's own size into the seed forces a full reseed — every
			// star in the field respawns at the new layout — the one frame
			// the viewport actually changes, and never lets a redraw at an
			// unchanged size touch it.
			viewportSeed := viewportSeed(viewport)
			if starfieldEffect != 0 {
				commands = append(commands, render.DrawParticles{
					BaseCommand: render.BaseCommand{LayerValue: engineSkyLayer},
					Effect:      starfieldEffect,
					Emitter:     1,
					Pos:         enginesplash.EngineStarfieldAnchor(viewport),
					EmitScale:   skyScale,
					Intensity:   alpha,
					Time:        elapsed,
					Dt:          dt,
					Burst:       1,
					Seed:        enginesplash.EngineStarfieldSeed(viewport),
				})
			}
			for _, star := range cruxStars {
				if star.effect == 0 {
					continue
				}
				intensity := enginesplash.EngineCruxIntensity(star.name, elapsed) * alpha
				if intensity <= 0 {
					continue
				}
				commands = append(commands, render.DrawParticles{
					BaseCommand: render.BaseCommand{LayerValue: engineSkyLayer},
					Effect:      star.effect,
					EmitScale:   cruxScale,
					Emitter:     1,
					Pos:         enginesplash.EngineCruxAnchor(viewport, star.name),
					Intensity:   intensity,
					Time:        elapsed,
					Dt:          dt,
					Seed:        uint64(star.effect) ^ viewportSeed,
				})
			}
		}

		return commands
	}
}

// manifestSplashParticles resolves a game-declared segment's optional
// "effect" reference into a fixed emitter at a fractional viewport anchor.
func manifestSplashParticles(assets asset.Manifest, decl asset.SplashDecl) func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	if decl.Effect == "" {
		return nil
	}
	effect := assets.Effect(decl.Effect)
	intensity := decl.EffectIntensity
	if intensity == 0 {
		intensity = 1
	}
	return func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
		if effect == 0 || alpha <= 0 {
			return nil
		}
		pos := math2d.Vec2{
			X: viewport.Min.X + viewport.Width()*decl.EffectAnchorX,
			Y: viewport.Min.Y + viewport.Height()*decl.EffectAnchorY,
		}
		return []render.DrawParticles{{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Effect:      effect,
			Emitter:     1,
			Pos:         pos,
			Intensity:   intensity * alpha,
			Time:        elapsed,
			Dt:          dt,
			Seed:        uint64(effect),
		}}
	}
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
	return ui.NewUpdateWithInput(&d.uiState, d.input.PointerFrame(), d.input.LastInput(), cam, d.uiTheme, d.cfg.Camera.Viewport)
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

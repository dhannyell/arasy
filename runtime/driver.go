package runtime

import (
	"log/slog"
	"math"

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
	mapper   *input.Mapper
	buffer   *input.Buffer
	controls devControls
	tick     uint64
	log      *slog.Logger

	// UI
	viewport math2d.Rect
	pointer  input.PointerState
	frame    input.PointerFrame
	uiState  ui.State
	uiTheme  ui.Theme

	lastInput        input.State
	speedAccumulator float64
	pendingReset     bool
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
		mapper:   input.NewMapper(cfg.InputBindings),
		buffer:   input.NewBuffer(),
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

// ApplyInput receives raw events from the platform and applies them to the mapper/buffer.
// Called by the platform before Advance, within the same lock.
func (d *Driver) ApplyInput(events []input.Event) {
	for _, event := range events {
		event.Tick = d.tick
		d.buffer.Push(event)

		if event.Kind != input.EventPointerMove && event.Kind != input.EventPointerLeave {
			d.mapper.Apply(event)
		}

		if event.Source.Device == input.DeviceMouse {
			d.accumulatePointer(event)
		}
	}
}

// Control applies a ControlCommand to the internal state.
// Called by the platform before Advance, within the same lock.
func (d *Driver) Control(cmd ControlCommand) {
	if d.controls.apply(cmd) {
		d.pendingReset = true
	}
}

// Advance executes a simulation tick.
// The platform calls this under an exclusive lock.
func (d *Driver) Advance(dt float64) {
	frame := d.controls.consume()

	if d.pendingReset {
		d.doReset()
		d.pendingReset = false
		// skips the reset tick (World has just been rebuilt)
		return
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
		return
	}

	stepsToRun := frame.steps
	if stepsToRun == 0 && frame.speed > 0 {
		d.speedAccumulator += frame.speed
		stepsToRun = int(math.Floor(d.speedAccumulator))
		d.speedAccumulator -= float64(stepsToRun)
	}

	if stepsToRun <= 0 {
		return
	}

	for range stepsToRun {
		if d.recorder != nil {
			d.recorder.Capture(d.world, d.mapper, d.lastInput, d.uiState, d.pointer, d.captureGameState(), d.tick)
		}

		d.lastInput = d.mapper.State()
		ctx := d.makeUpdateContext(d.newUpdateUI())
		d.game.Update(&ctx, dt)
		d.resetFrame()

		d.tick++
		d.mapper.EndFrame()
		d.buffer.ClearBefore(inputRetainFrom(d.tick, 120))
	}

	d.log.Debug("advance", "tick", d.tick, "checksum", d.world.Checksum())
}

func (d *Driver) Commands() []render.Command {
	uiInst := d.newRenderUI()
	ctx := d.makeRenderContext(uiInst)
	cmds := d.game.Render(&ctx)
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
	cmds := d.game.Render(&ctx)
	cmds = append(cmds, uiInst.Commands()...)
	return Frame{
		Camera:   ctx.Camera(),
		Commands: cmds,
	}
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
	return ui.New(
		&d.uiState,
		d.pointer.Pos(),
		d.pointer.Inside(),
		cam,
		d.uiTheme,
		d.cfg.Camera.Viewport,
	)
}

// newUpdateUI builds the interaction face of the UI, used in Update.
func (d *Driver) newUpdateUI() *ui.UI {
	cam := d.cameraForSurface()
	return ui.NewUpdate(&d.uiState, d.frame, cam, d.uiTheme, d.cfg.Camera.Viewport)
}

// SetViewport updates the surface size used for camera fitting and UI layout.
func (d *Driver) SetViewport(viewport math2d.Rect) {
	d.viewport = viewport
}

// resetFrame clears the per-tick PointerFrame after Update consumes it.
func (d *Driver) resetFrame() {
	d.frame = input.PointerFrame{
		Pos:     d.pointer.Pos(),
		Inside:  d.pointer.Inside(),
		Buttons: d.pointer.Buttons(),
	}
}

func (d *Driver) makeUpdateContext(uiInst *ui.UI) UpdateContext {
	return UpdateContext{
		World:      d.world,
		Input:      d.lastInput,
		Pointer:    d.frame,
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
		Debug:  d.controls.debugLayers,
		Log:    d.log.With("scope", "game"),
		Tick:   d.tick,
		camera: d.cameraForSurface(),
	}
}

// accumulatePointer folds a mouse event into the current tick's PointerFrame
// (coalesce) and updates the Driver's persistent pointer state.
func (d *Driver) accumulatePointer(e input.Event) {
	pos := math2d.Vec2{X: e.X, Y: e.Y}

	switch e.Kind {
	case input.EventPointerMove:
		d.frame.Pos = pos
		d.frame.Inside = true
		d.pointer.SetPos(pos)
		d.pointer.SetInside(true)
	case input.EventButtonDown:
		if btn, ok := mouseButton(e.Source.Code); ok {
			d.frame.Pos = pos
			d.frame.Inside = true
			d.frame.Buttons[btn] = true
			d.frame.Events = append(d.frame.Events, input.PointerEvent{
				Kind: e.Kind, Button: btn, Pos: pos,
			})
			d.pointer.SetPos(pos)
			d.pointer.SetButton(btn, true)
			d.pointer.SetInside(true)
		}
	case input.EventButtonUp:
		if btn, ok := mouseButton(e.Source.Code); ok {
			d.frame.Pos = pos
			d.frame.Inside = true
			d.frame.Buttons[btn] = false
			d.frame.Events = append(d.frame.Events, input.PointerEvent{
				Kind: e.Kind, Button: btn, Pos: pos,
			})
			d.pointer.SetPos(pos)
			d.pointer.SetButton(btn, false)
			d.pointer.SetInside(true)
		}
	case input.EventPointerLeave:
		d.frame.Inside = false
		for btn, down := range d.pointer.Buttons() {
			if down {
				d.frame.Buttons[btn] = false
				d.frame.Events = append(d.frame.Events, input.PointerEvent{
					Kind: input.EventButtonUp, Button: input.PointerButton(btn), Pos: d.pointer.Pos(),
				})
				d.pointer.SetButton(input.PointerButton(btn), false)
			}
		}
		d.pointer.SetInside(false)
	default:
		d.frame.Inside = false
	}
}

func mouseButton(code input.SourceCode) (input.PointerButton, bool) {
	switch code {
	case input.MouseLeft:
		return input.PointerPrimary, true
	case input.MouseRight:
		return input.PointerSecondary, true
	case input.MouseMiddle:
		return input.PointerMiddle, true
	default:
		return input.PointerUnknown, false
	}
}

// doReset rebuilds the World and calls Game.Init. Never called directly by the
// platform — it goes through Control(ControlReset) → pendingReset → Advance.
func (d *Driver) doReset() {
	d.world = scene.NewWorld()
	d.tick = 0
	d.lastInput = input.NewState()
	d.speedAccumulator = 0
	d.uiState = ui.State{}
	d.resetFrame()
	d.mapper.Reset()
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
	snap, ok := d.recorder.BackToTick(target)
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
	d.mapper = snap.mapper
	d.lastInput = snap.lastInput
	d.uiState = snap.uiState
	d.pointer = snap.pointer
	d.resetFrame()
	d.tick = snap.tick

	if game, ok := d.game.(StatefulGame); ok {
		game.RestoreGameState(snap.gameState)
	}
}

func (d *Driver) captureGameState() GameState {
	game, ok := d.game.(StatefulGame)
	if !ok {
		return nil
	}

	// StateSlot[T].GameState() already returns an independent value snapshot
	// (boxing a value type into the GameState interface copies it) — no
	// further cloning needed here, unlike world/mapper/uiState/pointer.
	return game.GameState()
}

func inputRetainFrom(tick, retainedTicks uint64) uint64 {
	if tick <= retainedTicks {
		return 0
	}
	return tick - retainedTicks
}

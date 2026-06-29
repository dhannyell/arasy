package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
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

	lastInput    input.State
	pendingReset bool
}

type DriveState struct {
	Tick        uint64
	Paused      bool
	Speed       int
	DebugLayers scene.DebugLayers
	HistoryLen  int
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
		d.mapper.Apply(event)
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

	if frame.paused && frame.steps == 0 {
		return
	}

	stepsToRun := frame.steps
	if stepsToRun == 0 {
		stepsToRun = frame.speed
	}

	if stepsToRun <= 0 {
		return
	}

	for range stepsToRun {
		if d.recorder != nil {
			d.recorder.Capture(d.world, d.mapper, d.lastInput, d.tick)
		}

		d.lastInput = d.mapper.State()
		ctx := d.makeContext()
		d.game.Update(&ctx, dt)

		d.tick++
		d.mapper.EndFrame()
		d.buffer.ClearBefore(inputRetainFrom(d.tick, 120))
	}

	d.log.Debug("advance", "tick", d.tick, "checksum", d.world.Checksum())
}

func (d *Driver) Commands() []render.Command {
	ctx := d.makeContext()
	return d.game.Render(&ctx)
}

func (d *Driver) State() DriveState {
	return DriveState{
		Tick:        d.tick,
		Paused:      d.controls.paused,
		Speed:       d.controls.speed,
		DebugLayers: d.controls.debugLayers,
		HistoryLen:  d.HistoryLen(),
	}
}

func (d *Driver) Config() Config { return d.cfg }
func (d *Driver) Tick() uint64   { return d.tick }
func (d *Driver) HistoryLen() int {
	if d.recorder == nil {
		return 0
	}
	return d.recorder.Len()
}
func (d *Driver) CurrentSpeed() int { return d.controls.speed }

func (d *Driver) Frame() Frame {
	ctx := d.makeContext()
	cmds := d.game.Render(&ctx)
	return Frame{
		Camera:   ctx.Camera,
		Commands: cmds,
	}
}

func (d *Driver) makeContext() Context {
	return Context{
		World:      d.world,
		Input:      d.lastInput,
		Log:        d.log.With("scope", "game"),
		Tick:       d.tick,
		Debug:      d.controls.debugLayers,
		Camera:     d.cfg.Camera,
		StepConfig: scene.StepConfig{Bounds: d.cfg.Bounds, Physics: d.cfg.Physics},
	}
}

func (d *Driver) context() Context {
	return Context{
		World: d.world,
		Log:   d.log.With("scope", "game"),
		Tick:  d.tick,
		StepConfig: scene.StepConfig{
			Bounds:  d.cfg.Bounds,
			Physics: d.cfg.Physics,
		},
	}
}

// doReset rebuilds the World and calls Game.Init. Never called directly by the
// platform — it goes through Control(ControlReset) → pendingReset → Advance.
func (d *Driver) doReset() {
	d.world = scene.NewWorld()
	d.tick = 0
	d.lastInput = input.NewState()
	d.mapper.Reset()
	if d.recorder != nil {
		d.recorder.Clear()
	}
	ctx := d.makeContext()
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
	d.world = snap.world
	d.mapper = snap.mapper
	d.lastInput = snap.lastInput
	d.tick = snap.tick

	d.log.Debug("back", "tick", d.tick)
}

func inputRetainFrom(tick, retainedTicks uint64) uint64 {
	if tick <= retainedTicks {
		return 0
	}
	return tick - retainedTicks
}

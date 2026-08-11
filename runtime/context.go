package runtime

import (
	"image"
	"log/slog"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

// UpdateContext is passed to Game.Init and Game.Update.
// World is mutable here; UI exposes deterministic interaction queries.
type UpdateContext struct {
	World *scene.World
	// Assets resolves texture and sprite names to render handles.
	Assets  asset.Manifest
	Input   input.State
	Pointer input.PointerFrame
	UI      ui.Interactor
	Log     *slog.Logger
	Tick    uint64
	Rand    *Rand
	Audio   *AudioPort
	// Batches publishes retained sprite/text batch content. The same port is
	// shared with RenderContext; publications drain at the next frame.
	Batches *BatchPort

	StepConfig scene.StepConfig

	// read-only; camera control belongs to Render
	camera         render.Camera
	simulationHeld bool
}

func (ctx *UpdateContext) Camera() render.Camera { return ctx.camera }

// HoldSimulation skips the automatic World.Step for this tick. Call it from
// every Update where simulation should remain frozen.
func (ctx *UpdateContext) HoldSimulation() { ctx.simulationHeld = true }

// RenderContext is passed to Game.Render.
// World is the read-only view of the simulation for this frame.
// Camera is changed through SetCamera, and UI only emits render commands.
type RenderContext struct {
	World scene.WorldView
	// Assets resolves texture and sprite names to render handles.
	Assets asset.Manifest
	UI     ui.Drawer
	// Time is the monotonic render-elapsed time in seconds. Not tied to
	// simulation ticks â€” use it for visual oscillations and UI animations.
	Time float64
	// Dt is the render-frame delta in seconds. Zero when called via Commands().
	// Use it for frame-rate-independent easing and UI animations.
	Dt   float64
	Log  *slog.Logger
	Tick uint64
	// Alpha is the normalized fraction [0, 1) between fixed simulation ticks.
	// It is 0 when render interpolation is disabled or unavailable.
	// Use it with World.RenderCommandsInterpolated to produce smooth visuals.
	Alpha float64
	// GamepadConnected reports whether a supported gamepad is currently
	// available to the host. Built-in hosts only recognize standard browser
	// mappings or devices mapped by GLFW. It is presentation-only: unlike
	// Input.LastInputDevice, it is not available to Update and never enters
	// snapshots, recordings, or checksums.
	GamepadConnected   bool
	UpdateTextureField *TexturePort
	// Batches publishes retained sprite/text batch content. Publications made
	// here are drained before the same frame's Render because RenderFrame
	// calls Frame() first.
	Batches *BatchPort

	// fixedDt is a simulation tick in seconds. 1/SimulationHz
	fixedDt float64

	camera render.Camera
	passes []render.Pass
}

func (ctx *RenderContext) Camera() render.Camera       { return ctx.camera }
func (ctx *RenderContext) SetCamera(cam render.Camera) { ctx.camera = cam }

// AddPass appends one extra camera and command stream to the frame. Extra
// passes are rendered before the base stream returned by Game.Render.
func (ctx *RenderContext) AddPass(camera render.Camera, commands []render.Command) {
	ctx.passes = append(ctx.passes, render.Pass{
		Camera:   camera,
		Commands: append([]render.Command(nil), commands...),
	})
}

// UpdateTexture publishes new content for a dynamic texture declared in the
// manifest. Ownership of img transfers to the engine -- do not mutate it
// after the call. Content is render-only: it never enters snapshots,
// checksums or recordings. Updates for unknown, non-dynamic, or
// wrong-dimension IDs are dropped with a warning; multiple updates for the
// same ID in one frame collapse to the last call.
func (ctx *RenderContext) UpdateTexture(id render.TextureID, img *image.RGBA) {
	if ctx.UpdateTextureField == nil {
		return
	}
	decl, ok := ctx.Assets.TextureDecl(id)
	if !ok || !decl.Dynamic {
		if ctx.Log != nil {
			ctx.Log.Warn("UpdateTexture: unknown or non-dynamic texture ID", "id", id)
		}
		return
	}
	b := img.Bounds()
	if b.Dx() != decl.W || b.Dy() != decl.H {
		if ctx.Log != nil {
			ctx.Log.Warn("UpdateTexture: dimension mismatch",
				"id", id, "want_w", decl.W, "want_h", decl.H, "got_w", b.Dx(), "got_h", b.Dy())
		}
		return
	}
	ctx.UpdateTextureField.UpdateTexture(id, img)
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
		fixedDt:            d.clock.DT(),
		camera:             d.cameraForSurface(),
	}
}

func (d *Driver) makeUpdateContext(uiInst *ui.UI) UpdateContext {
	return UpdateContext{
		World:      d.world,
		Assets:     d.cfg.Assets,
		Input:      d.input.LastInput(),
		Pointer:    d.input.PointerFrame(),
		Log:        d.log.With("scope", "game"),
		Tick:       d.tick,
		Rand:       &d.rng,
		UI:         uiInst.Interact(),
		Audio:      &d.audioPort,
		Batches:    &d.batchPort,
		StepConfig: scene.StepConfig{Bounds: d.cfg.Bounds, Bounce: d.cfg.Bounce, Physics: d.cfg.Physics},
		camera:     d.cameraForSurface(),
	}
}

// SubTick returns the current simulation position in fractional ticks.
// Use it for presentation values that need sub-tick updates beyond transform
// interpolation. When interpolation is disabled, it equals float64(Tick).
func (ctx *RenderContext) SubTick() float64 {
	return float64(ctx.Tick) + ctx.Alpha
}

// TicksSince returns the elapsed simulation ticks since start, including the
// current fractional tick. It returns zero when start is in the future.
func (ctx *RenderContext) TicksSince(start uint64) float64 {
	if start > ctx.Tick {
		return 0
	}
	return ctx.SubTick() - float64(start)
}

// SimTime returns the current simulation time in seconds. It stays aligned
// with the simulation during rewind and replay.
func (ctx *RenderContext) SimTime() float64 {
	return ctx.SubTick() * ctx.fixedDt
}

// SimTimeSince returns the signed seconds elapsed since tick on the
// simulation clock. Negative means tick is still in the future -- useful
// for countdowns anchored to a tick stored in simulation state.
func (ctx *RenderContext) SimTimeSince(tick uint64) float64 {
	return (ctx.SubTick() - float64(tick)) * ctx.fixedDt
}

package runtime

import (
	"image"
	"log/slog"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
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
// Treat World as read-only here; mutating it breaks determinism.
// Camera is changed through SetCamera, and UI only emits render commands.
type RenderContext struct {
	World *scene.World
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
	Alpha              float64
	UpdateTextureField *TexturePort

	camera render.Camera
}

func (ctx *RenderContext) Camera() render.Camera       { return ctx.camera }
func (ctx *RenderContext) SetCamera(cam render.Camera) { ctx.camera = cam }
func (ctx *RenderContext) UpdateTexture(id render.TextureID, img *image.RGBA) {
	if ctx.UpdateTextureField != nil {
		ctx.UpdateTextureField.UpdateTexture(id, img)
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
		UI:         uiInst,
		Audio:      &d.audioPort,
		StepConfig: scene.StepConfig{Bounds: d.cfg.Bounds, Bounce: d.cfg.Bounce, Physics: d.cfg.Physics},
		camera:     d.cameraForSurface(),
	}
}

package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

// UpdateContext is passed to Game.Init and Game.Update.
// World is mutable here; UI exposes deterministic interaction queries.
type UpdateContext struct {
	World   *scene.World
	Input   input.State
	Pointer input.PointerFrame
	UI      ui.Interactor
	Log     *slog.Logger
	Tick    uint64

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
	UI    ui.Drawer
	Time  float64
	Dt    float64
	Log   *slog.Logger
	Tick  uint64

	camera render.Camera
}

func (ctx *RenderContext) Camera() render.Camera       { return ctx.camera }
func (ctx *RenderContext) SetCamera(cam render.Camera) { ctx.camera = cam }

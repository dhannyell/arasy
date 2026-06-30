package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

// UpdateContext is passed to Game.Init and Game.Update. World is mutable—it is the
// only phase where the simulation state changes. UI is the interaction layer:
// deterministic queries derived from the tick's logged PointerFrame.
type UpdateContext struct {
	World   *scene.World
	Input   input.State
	Pointer input.PointerFrame
	UI      ui.Interactor
	Log     *slog.Logger
	Tick    uint64

	StepConfig scene.StepConfig

	// read-only; camera control belongs to Render
	camera render.Camera
}

func (ctx *UpdateContext) Camera() render.Camera { return ctx.camera }

// RenderContext is passed to Game.Render. World is conventionally
// read-only here (not yet enforced by the type system — phase (b) of the
// phasing described above); mutating it silently breaks the determinism
// contract. Camera is accessed via Camera() and adjusted via SetCamera() —
// never a directly writable field. UI acts as the drawing interface:
// methods are pure functions of World + cursor, producing render.Command.
type RenderContext struct {
	World *scene.World
	UI    ui.Drawer
	Debug scene.DebugLayers
	Time  float64
	Dt    float64
	Log   *slog.Logger
	Tick  uint64

	camera render.Camera
}

func (ctx *RenderContext) Camera() render.Camera       { return ctx.camera }
func (ctx *RenderContext) SetCamera(cam render.Camera) { ctx.camera = cam }

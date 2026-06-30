package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

// Context is passed to the Game on every tick.
// The Game never knows which platform is driving it.
type Context struct {
	World   *scene.World
	Input   input.State
	Pointer input.PointerFrame
	Log     *slog.Logger
	Tick    uint64
	Debug   scene.DebugLayers
	Camera  render.Camera
	UI      *ui.UI
	Time    float64
	Dt      float64

	StepConfig scene.StepConfig
}

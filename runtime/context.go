package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

// Context is passed to the Game on every tick.
// The Game never knows which platform is driving it.
type Context struct {
	World  *scene.World
	Input  input.State
	Log    *slog.Logger
	Tick   uint64
	Debug  scene.DebugLayers
	Camera render.Camera

	StepConfig scene.StepConfig
}

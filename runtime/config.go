package runtime

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

type Config struct {
	Camera        render.Camera
	Clear         render.Color
	Bounds        math2d.Rect
	Bounce        scene.BounceConfig
	Physics       scene.PhysicsConfig
	SimulationHz  int
	FPS           int
	InputBindings []input.Binding

	// Max ticks recorded.
	HistoryLen int
	// Seed initializes the driver's deterministic RNG (UpdateContext.Rand).
	// Zero is a valid, deterministic seed. The seed must be stable for the
	// lifetime of a game instance: rewind/scrub re-derive the stream from it.
	Seed uint64

	UITheme ui.Theme

	DebugOverlay scene.DebugOverlayConfig
}

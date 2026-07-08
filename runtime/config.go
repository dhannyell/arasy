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
	// KeyframeInterval is the spacing in ticks between full snapshots.
	// The default value is 30
	KeyframeInterval int

	UITheme ui.Theme

	// RenderInterpolation enables smooth visual interpolation between fixed
	// simulation ticks. When true, RenderContext.Alpha contains the accumulator
	// fraction [0, 1) and World.RenderCommandsInterpolated should be used
	// instead of World.RenderCommands. Default is false, preserving the
	// discrete-step look of existing games.
	RenderInterpolation bool

	DebugOverlay scene.DebugOverlayConfig
}

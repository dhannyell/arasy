package runtime

import (
	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

type Config struct {
	Camera       render.Camera
	Clear        render.Color
	Bounds       math2d.Rect
	Bounce       scene.BounceConfig
	Physics      scene.PhysicsConfig
	SimulationHz int
	// GamepadDeadzone is applied to gamepad stick axes in the deterministic
	// input mapper. Zero selects input.DefaultGamepadDeadzone.
	GamepadDeadzone float64
	// FPS caps the host frame rate. Only works in native hosts.
	// In WASM host vsync is forced by the browser, the rate is limited by user monitor rate.
	//
	// Zero (the zero value) uses the engine default(60); a positive value caps
	// at that many frames per second; a negative value runs the host loop uncapped.
	// The fixed simulation rate is SimulationHz and is independent of this cap.
	FPS int

	// InputBindings maps input events to game actions.
	InputBindings []input.Binding

	// HistoryLen caps the dense keyframe window in ticks. Restoring a keyframe
	// also requires the game to implement StatefulGame, usually by embedding a
	// StateSlot.
	HistoryLen int
	// KeyframeArchiveBudget caps thinned keyframes retained outside the dense
	// HistoryLen window. Zero uses the default (4x the dense window, minimum
	// 16). A negative value disables the archive, so seeks before the dense
	// window replay from tick 0.
	KeyframeArchiveBudget int
	// Assets is the resolved asset manifest. Games load it with asset.Load
	// (typically from an embed.FS) before constructing the game, and
	// resolve handles via the Assets field on the contexts.
	Assets asset.Manifest
	// Seed initializes the driver's deterministic RNG (UpdateContext.Rand).
	// Zero is a valid, deterministic seed. The seed must be stable for the
	// lifetime of a game instance: rewind/scrub re-derive the stream from it.
	Seed uint64
	// KeyframeInterval is the spacing in ticks between full snapshots.
	// The default value is 30
	KeyframeInterval int

	// UITheme is the theme shared by all UI elements. The default is ui.DefaultTheme.
	UITheme ui.Theme

	// RenderInterpolation enables smooth visual interpolation between fixed
	// simulation ticks. When true, RenderContext.Alpha contains the accumulator
	// fraction [0, 1) and World.RenderCommandsInterpolated should be used
	// instead of World.RenderCommands. Default is false, preserving the
	// discrete-step look of existing games.
	RenderInterpolation bool

	DebugOverlay scene.DebugOverlayConfig

	// ParticleOffscreenScale, when > 0, makes GPU-accelerated hosts render
	// particles into a dedicated non-multisampled offscreen target scaled by
	// this factor (1 = full resolution, 0.5 = half) and composite each
	// contiguous particle run back at its painter-order position. This trades
	// particle edge multisampling — irrelevant for small or textured
	// particles — for a large fill-rate reduction on heavy effects. Zero, the
	// default, keeps particles in the main pass; software renderers ignore
	// this field.
	ParticleOffscreenScale float64

	// DisableDefaultSplash zero value shows the engine logo; set to skip it.
	DisableDefaultSplash bool

	// BootBackground paints behind manifest-declared splash segments before
	// tick 0. The zero value uses Clear, so disabling the engine brand never
	// introduces an implicit black frame.
	BootBackground render.Color
}

package ui

// Space selects which coordinate system a widget's rect/position is
// expressed in.
type Space uint8

const (
	// SpaceRef anchors to an independent design-resolution canvas
	// (the reference passed at construction), letterboxed into the physical
	// viewport (AspectFit). Resolution independent, but only guaranteed to
	// stay glued to world-drawn content when the reference shares World's
	// aspect ratio. Default — use for HUD that doesn't need to track the
	// playfield (most menus, generic panels).
	SpaceRef Space = iota

	// SpacePlayfield anchors inside the actual on-screen rect where World is
	// rendered, resolved through the game camera's own Aspect strategy. The
	// reference canvas is fit into that rect instead of the raw viewport, so
	// widgets can never visually leave the playfield — even when the
	// reference resolution's aspect doesn't match World's. Use for anything
	// that must never cross world-drawn borders/content (score bar, minimap,
	// off-screen indicators).
	SpacePlayfield

	// SpaceScreen anchors directly in physical viewport pixels — no scaling,
	// no letterboxing. Escape hatch for pixel-exact, corner-pinned overlays
	// (pause icon, debug HUD) that intentionally ignore both the world and
	// the reference canvas.
	SpaceScreen
)

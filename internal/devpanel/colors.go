//go:build arr_dev

package devpanel

import "github.com/dhannyell/arasy/render"

// The panel needs a few colours the theme has no vocabulary for: "this is
// late", "this is wrong", "the collector ran here". They are literals here
// rather than theme entries because adding them to ui.Theme would put a debug
// overlay's palette in every game's theme, and a game has no use for any of
// them.
var (
	// colorWarning marks something outside its budget but still working.
	colorWarning = render.RGBA(0.95, 0.55, 0.25, 1)
	// colorCollected marks a frame the garbage collector ran during. It is
	// distinct from colorWarning because the two say different things: late
	// is the symptom, a collection is a cause, and a frame can be either
	// without being the other.
	colorCollected = render.RGBA(0.70, 0.50, 0.95, 1)
	// colorDanger marks a broken invariant. It is used for exactly one thing,
	// and that thing should be impossible to miss.
	colorDanger = render.RGBA(0.90, 0.25, 0.25, 1)
	// colorAction marks something a click can act on, against rows that are
	// only read.
	colorAction = render.RGBA(0.45, 0.75, 0.95, 1)
)

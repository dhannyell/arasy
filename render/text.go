package render

import "github.com/dhannyell/arasy/math2d"

const (
	textAdvanceRatio    = 0.6
	textLineHeightRatio = 1.25
)

// MeasureText returns the canonical engine text metrics.
//
// Text layout is backend-independent: callers can align columns and controls
// without depending on a platform renderer's native font metrics.
func MeasureText(text string, size float64) math2d.Vec2 {
	if size <= 0 {
		return math2d.Vec2{}
	}

	return math2d.Vec2{
		X: float64(len([]rune(text))) * size * textAdvanceRatio,
		Y: size * textLineHeightRatio,
	}
}

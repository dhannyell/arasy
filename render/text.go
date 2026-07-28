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
	return MeasureTextSpaced(text, size, 0)
}

// MeasureTextSpaced returns canonical metrics with world-space letter spacing
// inserted between adjacent runes.
func MeasureTextSpaced(text string, size, letterSpacing float64) math2d.Vec2 {
	if size <= 0 {
		return math2d.Vec2{}
	}

	count := len([]rune(text))
	width := float64(count) * size * textAdvanceRatio
	if count > 1 {
		width += float64(count-1) * letterSpacing
	}
	return math2d.Vec2{
		X: width,
		Y: size * textLineHeightRatio,
	}
}

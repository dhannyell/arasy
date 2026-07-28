package render

import (
	"unicode/utf8"

	"github.com/dhannyell/arasy/math2d"
)

const (
	textAdvanceRatio    = 0.6
	textLineHeightRatio = 1.25
)

// TextMetrics is a reusable canonical measurement for one text value, size,
// and letter spacing combination.
type TextMetrics struct {
	Size         math2d.Vec2
	NaturalWidth float64
	RuneCount    int
}

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
	return MeasureTextMetrics(text, size, letterSpacing).Size
}

// MeasureTextMetrics returns canonical size and rune count in one pass.
func MeasureTextMetrics(text string, size, letterSpacing float64) TextMetrics {
	if size <= 0 {
		return TextMetrics{}
	}

	count := utf8.RuneCountInString(text)
	width := float64(count) * size * textAdvanceRatio
	if count > 1 {
		width += float64(count-1) * letterSpacing
	}
	return TextMetrics{
		Size: math2d.Vec2{
			X: width,
			Y: size * textLineHeightRatio,
		},
		NaturalWidth: float64(count) * size * textAdvanceRatio,
		RuneCount:    count,
	}
}

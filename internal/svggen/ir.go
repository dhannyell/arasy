package svggen

import "github.com/dhannyell/arasy/math2d"

const (
	MaxInputBytes    = 4 << 20
	MaxDepth         = 64
	MaxElements      = 10_000
	MaxPathCommands  = 100_000
	MaxGeneratedData = 1_000_000
	MaxCSSRules      = 10_000
	MaxDataURLBytes  = 8 << 20
	MaxMagnitude     = 1e9
)

type Options struct {
	Strict bool
	Fonts  map[string]string
}

type Document struct {
	ViewBox math2d.Rect
	Root    *Element
	Styles  Stylesheet
	Fonts   map[string]string
}

type Element struct {
	Name       string
	Attributes map[string]string
	Children   []*Element
	Text       string
	Line       int
}

type Stylesheet struct {
	Rules     []Rule
	Keyframes map[string][]Keyframe
}

type Rule struct {
	Selector     string
	Declarations map[string]string
}

type Keyframe struct {
	Progress     float64
	Declarations map[string]string
}

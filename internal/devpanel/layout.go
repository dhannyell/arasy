package devpanel

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/ui"
)

const (
	// The panel is a fixed-size card pinned to the top-right corner. Fixed,
	// because a tooling overlay that reflows with the window would move its
	// buttons out from under the pointer every time the player resized.
	panelWidth  = 360
	panelHeight = 302
	panelMargin = 8

	contentPad   = 8
	rowHeight    = 26
	rowGap       = 6
	footerHeight = 42
)

// layout is the panel's geometry for one frame. Update and Render both derive
// it from the same function, which is what keeps a button drawn where it can
// be clicked: the two passes never compute a rect twice.
type layout struct {
	panel   math2d.Rect
	headers []math2d.Rect
	body    math2d.Rect
	footer  math2d.Rect
}

func (p *Panel) layout(viewport math2d.Rect, tabs int) layout {
	outer := ui.Inset(viewport, panelMargin)
	width := math.Min(panelWidth, outer.Width())
	height := math.Min(panelHeight, outer.Height())
	rect := math2d.NewRect(
		math2d.Vec2{X: outer.Max.X - width, Y: outer.Min.Y},
		math2d.Vec2{X: width, Y: height},
	)

	_, rest := ui.CutTop(rect, p.theme.Header.Height)
	bar, rest := ui.CutTop(rest, p.theme.Tabs.BarHeight)
	footer, body := ui.CutBottom(rest, footerHeight)

	return layout{
		panel:   rect,
		headers: ui.Columns(bar, tabs, 0),
		body:    ui.Inset(body, contentPad),
		footer:  ui.InsetXY(footer, contentPad, contentPad/2),
	}
}

// rows walks a rect top to bottom one row at a time. Every tab lays itself
// out this way, so a row added to one of them costs a line and shifts nothing
// below it by hand.
type rows struct {
	rest   math2d.Rect
	height float64
	gap    float64
}

func newRows(rect math2d.Rect) *rows {
	return &rows{rest: rect, height: rowHeight, gap: rowGap}
}

func (r *rows) next() math2d.Rect {
	piece, rest := ui.CutTop(r.rest, r.height)
	_, r.rest = ui.CutTop(rest, r.gap)
	return piece
}

// remaining is what is left below the rows taken so far.
func (r *rows) remaining() math2d.Rect { return r.rest }

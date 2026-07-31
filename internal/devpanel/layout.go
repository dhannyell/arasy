package devpanel

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/ui"
)

const (
	// The panel is a fixed-size card pinned to the top-right corner. Fixed in
	// its own space, not in pixels: the numbers below are authored against
	// designHeight and scale from there, so the panel keeps the same share of
	// the window at 1080p, at 4K and in fullscreen instead of shrinking into
	// a corner.
	panelWidth  = 360
	panelHeight = 302
	panelMargin = 8

	contentPad   = 8
	rowHeight    = 26
	rowGap       = 6
	footerHeight = 42

	// designHeight is the window height the sizes above are authored for. A
	// taller window scales everything up by the ratio; a shorter one does not
	// scale down, because shrinking the text is worse than letting the panel
	// take more of a small window.
	designHeight = 900
)

// panelScale is how many pixels one unit of panel space is worth.
func panelScale(viewport math2d.Rect) float64 {
	if viewport.Height() <= 0 {
		return 1
	}
	return math.Max(1, viewport.Height()/designHeight)
}

// panelSpace is the panel's coordinate space: the viewport divided by the
// scale. Deriving it from the viewport rather than fixing it keeps its aspect
// ratio equal to the window's, so ui's AspectFit reference space resolves to
// a pure uniform scale — no letterbox, and the panel's top-right corner is
// still the window's top-right corner on an ultrawide.
func panelSpace(viewport math2d.Rect) math2d.Rect {
	scale := panelScale(viewport)
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{
		X: viewport.Width() / scale,
		Y: viewport.Height() / scale,
	})
}

// layout is the panel's geometry for one frame. Update and Render both derive
// it from the same function, which is what keeps a button drawn where it can
// be clicked: the two passes never compute a rect twice.
type layout struct {
	panel   math2d.Rect
	headers []math2d.Rect
	body    math2d.Rect
	footer  math2d.Rect
}

// layout places the panel inside space, which is panelSpace(viewport) — not
// the viewport itself. Everything below is therefore in panel units, and the
// scaling happens once, where the space is built.
func (p *Panel) layout(space math2d.Rect, tabs int) layout {
	outer := ui.Inset(space, panelMargin)
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

// frameLayout is the panel's geometry for a window of this size, the entry
// point both passes use so neither can forget to scale.
func (p *Panel) frameLayout(viewport math2d.Rect) layout {
	return p.layout(panelSpace(viewport), len(p.tabs()))
}

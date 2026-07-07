package ui

import (
	"math"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// ListDraw draws the content of one row already positioned at rowRect (in the
// same space List/ListIn was called with). Rows outside the visible scroll
// window are culled before draw is called.
type ListDraw func(row int, rowRect math2d.Rect)

// List is a scrollable list of freeform rows: unlike Table it has no columns
// or header — each row's content is emitted by draw. List handles scrolling,
// culling, zebra/hover/selected backgrounds and click hit-testing, so games
// don't have to hand-roll the "offset, cull, rebuild row rect" pattern for
// every row-based screen (squad lists, calendars, inventories, etc).
func (u *UI) List(id string, rect math2d.Rect, rowH float64, rowCount, selected int, draw ListDraw) int {
	return u.ListIn(id, rect, rowH, rowCount, selected, draw, SpaceRef)
}

// ListHitIn is called in Update. It returns the clicked row index, or -1.
func (u *UI) ListHitIn(id string, rect math2d.Rect, rowH float64, rowCount int, space Space) int {
	wid := HashID(id)
	offset := 0.0
	if u.state.scroll != nil {
		offset = u.state.scroll[wid]
	}

	clicked := -1
	for _, e := range u.frame.Events {
		if e.Button != input.PointerPrimary {
			continue
		}
		pos := u.cameraFor(space).ScreenToWorld(e.Pos)
		switch e.Kind {
		case input.EventButtonDown:
			if rect.Contains(pos) && u.state.activeID == noID {
				u.state.activeID = wid
				u.state.focusID = wid
			}
		case input.EventButtonUp, input.EventPointerLeave:
			if u.state.activeID != wid {
				continue
			}
			if e.Kind == input.EventButtonUp && rect.Contains(pos) {
				row := int(math.Floor((pos.Y - rect.Min.Y + offset) / rowH))
				if row >= 0 && row < rowCount {
					clicked = row
				}
			}
			u.state.activeID = noID
		}
	}
	return clicked
}

// ListIn draws the scrollable row window (background + draw callback per
// visible row) and returns the clicked row index, or -1.
func (u *UI) ListIn(id string, rect math2d.Rect, rowH float64, rowCount, selected int, draw ListDraw, space Space) int {
	rowCount = max(rowCount, 0)
	contentH := float64(rowCount) * rowH
	offset, clip := u.ScrollAreaIn(id, rect, contentH, space)

	first := max(int(math.Floor(offset/rowH)), 0)
	last := min(int(math.Ceil((offset+rect.Height())/rowH)), rowCount-1)

	cursor := u.cursorIn(space)
	for row := first; row <= last; row++ {
		rowRect := math2d.NewRect(
			math2d.Vec2{X: rect.Min.X, Y: rect.Min.Y + float64(row)*rowH - offset},
			math2d.Vec2{X: rect.Width(), Y: rowH},
		)

		// The row at the top/bottom edge of the scroll window is often only
		// partially visible; clip everything the row emits (the background
		// fill and whatever draw adds, neither of which know about the
		// scroll window) so content is cut cleanly at the list bounds
		// instead of bleeding past them.
		start := len(u.commands)

		fill := u.theme.Row.Normal
		if row%2 == 1 {
			fill = u.theme.Row.Zebra
		}
		if row == selected {
			fill = u.theme.Row.Selected
		} else if u.inside && rowRect.Contains(cursor) {
			fill = u.theme.Row.Hovered
		}
		if fill.A > 0 {
			u.FillRectIn(rowRect, fill, space)
		}

		if draw != nil {
			draw(row, rowRect)
		}

		render.ClipCommands(u.commands[start:], clip)
	}

	return u.ListHitIn(id, rect, rowH, rowCount, space)
}

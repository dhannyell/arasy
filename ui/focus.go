package ui

import (
	"math"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Keyboard navigation follows the order widgets are declared in: an immediate
// UI is rebuilt every frame, so the order the game writes is the order the
// player tabs through, with nothing to keep in sync. Tab cycles, Enter
// activates the focused widget down the same path a click takes, and the
// arrows move the selection of a focused list or table.
//
// The engine only claims those keys when the frame declared something
// focusable. A game with no UI up keeps Tab and Enter for itself.
//
// Two keys stay with the game on purpose. Space is not an activation key here:
// it is a jump, a pause, a shot in too many games for the UI to take it by
// default. Shift+Tab has no backward step yet, because modifier keys are not
// part of the input vocabulary — when they arrive, this is where the reverse
// cycle goes.

// focusedIn reports whether id holds keyboard focus right now. The themed
// widgets draw their own focus ring, so this exists for the chrome a game
// draws itself: a button with a shape the engine does not know how to outline
// still has to show the player where the keyboard is. Without it the focus
// state is invisible outside this package, since ui.State is private.
func (u *UI) focusedIn(id string) bool {
	return u.state != nil && u.state.focusID == HashID(id)
}

// focusRingIn draws the themed focus ring around rect. Every focusable widget
// in this package draws the same ring, so keyboard focus reads the same
// everywhere; a game drawing its own widget can reach it through
// DrawScope.FocusRing.
func (u *UI) focusRingIn(rect math2d.Rect, layer int, space Space) {
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(insetRect(rect, -u.theme.Focus.Inset), space),
		Stroke:       u.theme.Focus.Color,
		StrokeWidth:  u.sizeToWorld(u.theme.Focus.Width, space),
		CornerRadius: u.sizeToWorld(u.theme.Focus.CornerRadius, space),
	})
}

// keyActs reports whether an editing or navigation key should act this tick:
// either it was pressed now, or it is held and the repeat clock fired. The
// answer is resolved once per frame and shared, so a text field and a focused
// table reading the same held key advance one clock between them rather than
// one each.
func (u *UI) keyActs(code input.SourceCode) bool {
	if u.keys.JustPressed(code) {
		u.state.repeatCode = code
		u.state.repeatTicks = 0
		return true
	}
	repeated, ok := u.keyRepeat()
	return ok && repeated == code
}

func (u *UI) keyRepeat() (input.SourceCode, bool) {
	if !u.repeatResolved {
		u.repeatCode, u.repeatFired = u.advanceKeyRepeat()
		u.repeatResolved = true
	}
	return u.repeatCode, u.repeatFired
}

// advanceKeyRepeat ticks the repeat clock of the key currently held and reports
// whether it fires this tick. Releasing the key drops the clock, so a key held
// across a rewind resumes from the restored count rather than from however long
// the player's finger has actually been down.
func (u *UI) advanceKeyRepeat() (input.SourceCode, bool) {
	code := u.state.repeatCode
	if code == input.KeyUnknown {
		return input.KeyUnknown, false
	}
	if !u.keys.Down(code) {
		u.state.repeatCode = input.KeyUnknown
		u.state.repeatTicks = 0
		return input.KeyUnknown, false
	}

	u.state.repeatTicks++
	if u.state.repeatTicks < textRepeatDelay {
		return code, false
	}
	return code, (u.state.repeatTicks-textRepeatDelay)%textRepeatRate == 0
}

// rowNavigation moves a row selection by the navigation keys pressed this tick.
// page is how many rows fit in the view, which is what PageUp/PageDown move by.
func (u *UI) rowNavigation(selected, rowCount, page int) int {
	if rowCount <= 0 {
		return -1
	}
	if u.keyActs(input.KeyArrowUp) {
		selected--
	}
	if u.keyActs(input.KeyArrowDown) {
		selected++
	}
	if u.keyActs(input.KeyPageUp) {
		selected -= max(page, 1)
	}
	if u.keyActs(input.KeyPageDown) {
		selected += max(page, 1)
	}
	if u.keyActs(input.KeyHome) {
		selected = 0
	}
	if u.keyActs(input.KeyEnd) {
		selected = rowCount - 1
	}
	return clampInt(selected, 0, rowCount-1)
}

// listInputValueIn is called in Update. It is the caller-owned selection of a
// row list: pass the index you hold, store the one returned. Clicks select, and
// so do the arrows while the list holds focus — which is the point of keeping
// the index with the caller, since both paths write the same place.
func (u *UI) listInputValueIn(id string, rect math2d.Rect, rowH float64, rowCount, selected int, space Space) (int, bool) {
	u.focusableIn(id, rect, space)
	next := selected

	if row := u.listHitIn(id, rect, rowH, rowCount, space); row >= 0 {
		next = row
	} else if u.state.focusID == HashID(id) {
		next = u.rowNavigation(next, rowCount, visibleRows(rect.Height(), rowH))
	}
	if next != selected {
		u.revealRow(id, rect, rowH, next, rowCount, space)
	}
	return next, next != selected
}

// tableInputValueIn is listInputValueIn for a table, measuring rows against the
// body below the header.
func (u *UI) tableInputValueIn(id string, rect math2d.Rect, cols []TableColumn, rowCount, selected int, space Space) (int, bool) {
	body := tableBodyRect(rect, u.theme.Header.Height)
	rowH := u.theme.Row.Height
	u.focusableIn(id, rect, space)
	next := selected

	if row := u.tableHitIn(id, rect, cols, rowCount, selected, space); row >= 0 {
		next = row
	} else if u.state.focusID == HashID(id) {
		next = u.rowNavigation(next, rowCount, visibleRows(body.Height(), rowH))
	}
	if next != selected {
		u.revealRow(id, body, rowH, next, rowCount, space)
	}
	return next, next != selected
}

// revealRow scrolls the view just far enough to show the row that was selected
// from the keyboard. Without it the selection walks off the visible window and
// the player is moving something they cannot see.
func (u *UI) revealRow(id string, view math2d.Rect, rowH float64, row, rowCount int, space Space) {
	_ = space
	if row < 0 || rowH <= 0 || view.Height() <= 0 {
		return
	}
	u.ensureScroll()

	wid := HashID(id)
	offset := u.state.scroll[wid]
	top := float64(row) * rowH
	bottom := top + rowH
	switch {
	case top < offset:
		offset = top
	case bottom > offset+view.Height():
		offset = bottom - view.Height()
	}
	u.state.scroll[wid] = clamp(offset, 0, maxScrollOffset(view, float64(rowCount)*rowH))
}

func visibleRows(height, rowH float64) int {
	if rowH <= 0 {
		return 1
	}
	return max(int(math.Floor(height/rowH)), 1)
}

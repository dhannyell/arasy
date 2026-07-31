package ui

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// hoveredIn reports whether the cursor is currently over rect. Unlike
// clickedIn/activatedIn it carries no persisted state — it is safe to call
// from Render as well as Update.
func (u *UI) hoveredIn(rect math2d.Rect, space Space) bool {
	return u.inside && rect.Contains(u.cursorIn(space))
}

// blockedAt reports whether a screen position belongs to an open popup owned
// by another widget. An overlay draws above everything, so it must also
// intercept above everything: without this the row a dropdown list covers
// would answer the same click that picked an item from the list.
func (u *UI) blockedAt(wid ID, screen math2d.Vec2) bool {
	if u.state == nil || u.state.popupOwner == noID || u.state.popupOwner == wid {
		return false
	}
	return u.state.popupRect.Contains(screen)
}

// clickedIn is called in Update. It returns true on the tick the click completes,
// and mutates state.activeID (captured in the snapshot). Deterministic.
func (u *UI) clickedIn(id string, rect math2d.Rect, space Space) bool {
	wid := HashID(id)
	clicked := false
	for _, e := range u.frame.Events {
		if e.Button != input.PointerPrimary || u.blockedAt(wid, e.Pos) {
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
			if u.state.activeID == wid {
				if e.Kind == input.EventButtonUp && rect.Contains(pos) {
					clicked = true
				}
				u.state.activeID = noID
			}
		default:
			continue
		}
	}
	return clicked
}

func (u *UI) activatedIn(id string, rect math2d.Rect, space Space) bool {
	wid := HashID(id)
	u.focusableIn(id, rect, space)
	return u.clickedIn(id, rect, space) || u.activated == wid
}

func (u *UI) focusableIn(id string, rect math2d.Rect, space Space) {
	_ = rect
	_ = space
	wid := HashID(id)
	for _, existing := range u.focusable {
		if existing == wid {
			return
		}
	}
	u.focusable = append(u.focusable, wid)
}

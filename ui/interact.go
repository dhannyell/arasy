package ui

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

// Hovered reports whether the cursor is currently over rect. Unlike
// Clicked/Activated it carries no persisted state — it is safe to call from
// Render as well as Update.
func (u *UI) Hovered(rect math2d.Rect) bool { return u.HoveredIn(rect, SpaceRef) }

func (u *UI) HoveredIn(rect math2d.Rect, space Space) bool {
	return u.inside && rect.Contains(u.cursorIn(space))
}

func (u *UI) Clicked(id string, rect math2d.Rect) bool { return u.ClickedIn(id, rect, SpaceRef) }

func (u *UI) Activated(id string, rect math2d.Rect) bool { return u.ActivatedIn(id, rect, SpaceRef) }

// ClickedIn is called in Update. It returns true on the tick the click completes,
// and mutates state.activeID (captured in the snapshot). Deterministic.
func (u *UI) ClickedIn(id string, rect math2d.Rect, space Space) bool {
	wid := HashID(id)
	clicked := false
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

func (u *UI) ActivatedIn(id string, rect math2d.Rect, space Space) bool {
	wid := HashID(id)
	u.FocusableIn(id, rect, space)
	return u.ClickedIn(id, rect, space) || u.activated == wid
}

func (u *UI) Focusable(id string, rect math2d.Rect) { u.FocusableIn(id, rect, SpaceRef) }

func (u *UI) FocusableIn(id string, rect math2d.Rect, space Space) {
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

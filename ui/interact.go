package ui

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

func (u *UI) Clicked(id string, rect math2d.Rect) bool { return u.ClickedIn(id, rect, SpaceRef) }

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

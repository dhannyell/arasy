package ui

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

// Clicked is called in Update. It returns true on the tick the click completes,
// and mutates state.activeID (captured in the snapshot). Deterministic.
func (u *UI) Clicked(id string, rect math2d.Rect) bool {
	wid := HashID(id)
	clicked := false
	for _, e := range u.frame.Events {
		if e.Button != input.PointerPrimary {
			continue
		}
		switch e.Kind {
		case input.EventButtonDown:
			if rect.Contains(e.Pos) && u.state.activeID == noID {
				u.state.activeID = wid
			}
		case input.EventButtonUp, input.EventPointerLeave:
			if u.state.activeID == wid {
				if e.Kind == input.EventButtonUp && rect.Contains(e.Pos) {
					clicked = true
				}
				u.state.activeID = noID
			}
		default:
			panic("unhandled default case")
		}
	}
	return clicked
}

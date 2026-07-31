package ui

import (
	"maps"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// State persists across frames and is CAPTURED in the driverSnapshot (cloned),
// ensuring release semantics survive a rewind. It belongs to the Driver.
type State struct {
	activeID ID
	focusID  ID
	scroll   map[ID]float64
	tabs     map[ID]int
	caret    map[ID]int
	open     map[ID]bool
	// repeatCode and repeatTicks are the engine-side auto-repeat clock of the
	// focused editing key: how a held backspace keeps deleting. Counting ticks
	// here rather than trusting the OS repeat rate is what makes a held key
	// behave the same live, in a replay and after a rewind.
	repeatCode  input.SourceCode
	repeatTicks int
	// popupOwner and popupRect are the open overlay's claim on the screen.
	// Widgets underneath an open dropdown must not answer clicks that landed
	// on the popup covering them, and only the owner knows the popup exists —
	// so the claim is parked here, where every widget can see it, in screen
	// coordinates because that is the one space they all share.
	popupOwner ID
	popupRect  math2d.Rect
}

// ActiveID reports the widget currently holding the pointer, or zero when
// none does. A widget stays active for as long as the press lasts, which is
// what makes it the right question for an overlay deciding whether to keep
// the mouse: a slider dragged past its own bounds is still being dragged, and
// a hit test against the overlay rect would answer no.
func (s *State) ActiveID() ID { return s.activeID }

// FocusID reports the widget currently holding keyboard focus, or zero when
// none does. An overlay reads it to decide whether the keys of this frame are
// being typed into it or belong to the game.
func (s *State) FocusID() ID { return s.focusID }

// Clone is used by the runtime driver when capturing rewind snapshots.
func (s State) Clone() State {
	return State{
		activeID:    s.activeID,
		focusID:     s.focusID,
		scroll:      maps.Clone(s.scroll),
		tabs:        maps.Clone(s.tabs),
		caret:       maps.Clone(s.caret),
		open:        maps.Clone(s.open),
		repeatCode:  s.repeatCode,
		repeatTicks: s.repeatTicks,
		popupOwner:  s.popupOwner,
		popupRect:   s.popupRect,
	}
}

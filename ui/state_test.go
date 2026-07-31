package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

func TestStateCloneCopiesScrollMap(t *testing.T) {
	areaID := HashID("roster")
	state := State{
		activeID: HashID("active"),
		focusID:  HashID("focus"),
		scroll:   map[ID]float64{areaID: 42},
	}

	clone := state.Clone()
	state.scroll[areaID] = 7

	if clone.activeID != state.activeID {
		t.Fatalf("clone activeID = %v, want %v", clone.activeID, state.activeID)
	}
	if clone.focusID != state.focusID {
		t.Fatalf("clone focusID = %v, want %v", clone.focusID, state.focusID)
	}
	if clone.scroll[areaID] != 42 {
		t.Fatalf("clone scroll = %v, want independent value 42", clone.scroll[areaID])
	}
}

// The active tab is UI state, so it has to be cloned with the rest of the
// snapshot — otherwise a rewind would leave the snapshot sharing the live map
// and the tab selection would not restore with it.
func TestStateCloneCopiesTabsMap(t *testing.T) {
	tabsID := HashID("inspector")
	state := State{tabs: map[ID]int{tabsID: 2}}

	clone := state.Clone()
	state.tabs[tabsID] = 0

	if clone.tabs[tabsID] != 2 {
		t.Fatalf("clone tabs = %v, want independent value 2", clone.tabs[tabsID])
	}
}

// Everything a text field, a dropdown or the repeat clock remembers has to
// rewind with the snapshot too — a caret restored to the wrong character would
// make the same replayed keystroke edit a different place.
func TestStateCloneCopiesWidgetState(t *testing.T) {
	fieldID := HashID("search")
	menuID := HashID("filter")
	state := State{
		caret:       map[ID]int{fieldID: 4},
		open:        map[ID]bool{menuID: true},
		repeatCode:  input.KeyBackspace,
		repeatTicks: 12,
		popupOwner:  menuID,
		popupRect:   math2d.NewRect(math2d.Vec2{X: 1, Y: 2}, math2d.Vec2{X: 3, Y: 4}),
	}

	clone := state.Clone()
	state.caret[fieldID] = 0
	state.open[menuID] = false

	if clone.caret[fieldID] != 4 {
		t.Fatalf("clone caret = %v, want independent value 4", clone.caret[fieldID])
	}
	if !clone.open[menuID] {
		t.Fatal("clone lost the open dropdown")
	}
	if clone.repeatCode != input.KeyBackspace || clone.repeatTicks != 12 {
		t.Fatalf("clone repeat clock = (%v, %d), want (backspace, 12)", clone.repeatCode, clone.repeatTicks)
	}
	if clone.popupOwner != menuID || clone.popupRect != state.popupRect {
		t.Fatalf("clone popup = (%v, %v), want the owner and rect", clone.popupOwner, clone.popupRect)
	}
}

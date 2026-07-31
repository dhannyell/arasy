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

// A tooling overlay decides whether it keeps this frame's mouse and keyboard
// by asking who is active and who is focused. Both answers have to track the
// live interaction, or a slider dragged outside the overlay would hand the
// drag back to the game mid-gesture.
func TestActiveAndFocusAccessorsFollowInteraction(t *testing.T) {
	state := &State{}
	rect := btnRect()
	cam := testInteractCamera()

	if state.ActiveID() != noID || state.FocusID() != noID {
		t.Fatalf("fresh state = (%v, %v), want no active or focused widget", state.ActiveID(), state.FocusID())
	}

	down := NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
	), cam, DefaultTheme(), cam.Viewport)
	down.clickedIn("btn", rect, SpaceRef)

	want := HashID("btn")
	if state.ActiveID() != want {
		t.Fatalf("ActiveID after press = %v, want %v", state.ActiveID(), want)
	}
	if state.FocusID() != want {
		t.Fatalf("FocusID after press = %v, want %v", state.FocusID(), want)
	}

	up := NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
	), cam, DefaultTheme(), cam.Viewport)
	up.clickedIn("btn", rect, SpaceRef)

	if state.ActiveID() != noID {
		t.Fatalf("ActiveID after release = %v, want none", state.ActiveID())
	}
	if state.FocusID() != want {
		t.Fatalf("FocusID after release = %v, want %v (focus outlives the press)", state.FocusID(), want)
	}
}

// The accessors read the snapshot they are called on, so a cloned state
// answers for the clone and not for the live one it was taken from.
func TestActiveAndFocusAccessorsSurviveClone(t *testing.T) {
	state := State{activeID: HashID("slider"), focusID: HashID("field")}
	clone := state.Clone()

	if clone.ActiveID() != state.ActiveID() || clone.FocusID() != state.FocusID() {
		t.Fatalf("clone = (%v, %v), want (%v, %v)",
			clone.ActiveID(), clone.FocusID(), state.ActiveID(), state.FocusID())
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

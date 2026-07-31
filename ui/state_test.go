package ui

import "testing"

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

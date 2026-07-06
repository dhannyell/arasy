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

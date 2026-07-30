package platforminput_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/platforminput"
)

func TestStickyGamepadSelectorSkipsSparseAndNonStandardSlots(t *testing.T) {
	var selector platforminput.StickyGamepadSelector

	// Index 0 represents a sparse/null slot; index 1 a connected gamepad
	// rejected by the browser adapter because its mapping is non-standard.
	index, ok := selector.Select([]bool{false, false, true, false})
	if !ok || index != 2 {
		t.Fatalf("selection = %d, %v, want index 2", index, ok)
	}
}

func TestStickyGamepadSelectorKeepsSelectedIndex(t *testing.T) {
	var selector platforminput.StickyGamepadSelector
	if index, ok := selector.Select([]bool{false, false, true}); !ok || index != 2 {
		t.Fatalf("initial selection = %d, %v, want index 2", index, ok)
	}

	index, ok := selector.Select([]bool{true, false, true})
	if !ok || index != 2 {
		t.Fatalf("sticky selection = %d, %v, want index 2", index, ok)
	}
}

func TestStickyGamepadSelectorWaitsOnePollBeforeReassignment(t *testing.T) {
	var selector platforminput.StickyGamepadSelector
	if index, ok := selector.Select([]bool{false, true}); !ok || index != 1 {
		t.Fatalf("initial selection = %d, %v, want index 1", index, ok)
	}

	if index, ok := selector.Select([]bool{true, false}); ok {
		t.Fatalf("disconnect selection = %d, %v, want absent frame", index, ok)
	}
	index, ok := selector.Select([]bool{true, false})
	if !ok || index != 0 {
		t.Fatalf("reassignment = %d, %v, want index 0", index, ok)
	}
}

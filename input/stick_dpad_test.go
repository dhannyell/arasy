package input_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
)

// The hysteresis is the whole point: a stick wobbling around either threshold
// or across the diagonal must not flicker the direction. Each scenario here
// is a wobble that a plain "dominant axis over one threshold" gets wrong.
func TestStickDPadHoldsDirectionThroughWobble(t *testing.T) {
	var pad input.StickDPad

	if dx, dy := pad.Update(0.4, 0); dx != 0 || dy != 0 {
		t.Fatalf("0.4 push engaged (%d,%d); engaging should take StickEngage", dx, dy)
	}
	if dx, _ := pad.Update(0.8, 0); dx != 1 {
		t.Fatalf("firm right push reads dx=%d, want 1", dx)
	}

	// Sagging below engage but above release keeps the direction held.
	if dx, _ := pad.Update(0.4, 0); dx != 1 {
		t.Fatalf("sag to 0.4 dropped the held direction (dx=%d)", dx)
	}

	// A diagonal wobble where Y creeps past X must not switch axes until it
	// clearly dominates.
	if dx, dy := pad.Update(0.5, 0.55); dx != 1 || dy != 0 {
		t.Fatalf("slight diagonal switched axes to (%d,%d)", dx, dy)
	}
	if dx, dy := pad.Update(0.3, 0.8); dx != 0 || dy != 1 {
		t.Fatalf("clear vertical push reads (%d,%d), want (0,1)", dx, dy)
	}

	// Reversing along the held axis flips without passing through neutral.
	if _, dy := pad.Update(0, -0.9); dy != -1 {
		t.Fatalf("reversal along the held axis reads dy=%d, want -1", dy)
	}

	if dx, dy := pad.Update(0.1, 0.1); dx != 0 || dy != 0 {
		t.Fatalf("near-neutral stick still reads (%d,%d)", dx, dy)
	}
}

package input_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
)

// The contract worth pinning: an immediate step on press, silence through the
// delay, then the interval cadence -- and a release rearms the immediate step.
func TestRepeaterStepsOnPressThenAfterTheDelay(t *testing.T) {
	r := input.Repeater{Delay: 0.3, Interval: 0.1}
	dt := 0.05

	if got := r.Step(true, dt); got != 1 {
		t.Fatalf("press fired %d steps, want the immediate 1", got)
	}

	steps := 0
	for range 5 { // 0.25s held: still inside the delay
		steps += r.Step(true, dt)
	}
	if steps != 0 {
		t.Fatalf("fired %d steps inside the delay, want 0", steps)
	}

	steps = 0
	for range 8 { // 0.4s more: the delay ends and the 0.1s cadence runs
		steps += r.Step(true, dt)
	}
	if steps != 4 {
		t.Fatalf("fired %d steps over 0.4s of cadence, want 4", steps)
	}

	if got := r.Step(false, dt); got != 0 {
		t.Fatalf("release fired %d steps, want 0", got)
	}
	if got := r.Step(true, dt); got != 1 {
		t.Fatalf("re-press fired %d steps, want the immediate 1 again", got)
	}
}

// A long tick must not swallow cadence: dt spanning several intervals reports
// them all, which is what keeps held movement identical live and in replay.
func TestRepeaterReportsEveryStepAcrossALongTick(t *testing.T) {
	r := input.Repeater{Delay: 0.1, Interval: 0.1}
	r.Step(true, 0) // press

	if got := r.Step(true, 0.35); got != 3 {
		t.Fatalf("a 0.35s tick fired %d steps, want 3", got)
	}
}

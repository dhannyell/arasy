package loop_test

import (
	"testing"

	"github.com/dhannyell/arasy/loop"
)

func TestStepsAccumulatesFractionalSpeed(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := 0.0

	got := loop.Steps(0.5, simDT, simDT, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("first plan = %+v, want 0 unclamped", got)
	}

	got = loop.Steps(0.5, simDT, simDT, &accumulator, 0)
	if got.Steps != 1 || got.Clamped {
		t.Fatalf("second plan = %+v, want 1 unclamped", got)
	}

	got = loop.Steps(0.5, simDT, simDT, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("third plan = %+v, want 0 unclamped", got)
	}
}

func TestStepsClampsAndDiscardsExcess(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := 0.0

	got := loop.Steps(1, 10*simDT, simDT, &accumulator, 5)
	if got.Steps != 5 || !got.Clamped {
		t.Fatalf("plan = %+v, want 5 clamped", got)
	}

	got = loop.Steps(1, 0, simDT, &accumulator, 5)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("plan after clamp = %+v, want 0 unclamped", got)
	}
}

func TestStepsSpeedZeroDoesNotAccumulate(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := simDT / 2

	got := loop.Steps(0, simDT, simDT, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("plan = %+v, want 0 unclamped", got)
	}
	if accumulator != simDT/2 {
		t.Fatalf("accumulator = %v, want %v", accumulator, simDT/2)
	}
}

func TestStepsMaxZeroDoesNotClamp(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := 0.0

	got := loop.Steps(1, 10*simDT, simDT, &accumulator, 0)
	if got.Steps != 10 || got.Clamped {
		t.Fatalf("plan = %+v, want 10 unclamped", got)
	}
}

func TestStepsIgnoresInvalidTiming(t *testing.T) {
	accumulator := 1.0

	got := loop.Steps(1, 1, 0, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("plan = %+v, want 0 unclamped", got)
	}
	if accumulator != 1.0 {
		t.Fatalf("accumulator = %v, want 1", accumulator)
	}
}

package runtime

import "testing"

func TestPlanStepsAccumulatesFractionalSpeed(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := 0.0

	got := planSteps(0.5, simDT, simDT, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("first plan = %+v, want 0 unclamped", got)
	}

	got = planSteps(0.5, simDT, simDT, &accumulator, 0)
	if got.Steps != 1 || got.Clamped {
		t.Fatalf("second plan = %+v, want 1 unclamped", got)
	}

	got = planSteps(0.5, simDT, simDT, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("third plan = %+v, want 0 unclamped", got)
	}
}

func TestPlanStepsClampsAndDiscardsExcess(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := 0.0

	got := planSteps(1, 10*simDT, simDT, &accumulator, 5)
	if got.Steps != 5 || !got.Clamped {
		t.Fatalf("plan = %+v, want 5 clamped", got)
	}

	got = planSteps(1, 0, simDT, &accumulator, 5)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("plan after clamp = %+v, want 0 unclamped", got)
	}
}

func TestPlanStepsSpeedZeroDoesNotAccumulate(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := simDT / 2

	got := planSteps(0, simDT, simDT, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("plan = %+v, want 0 unclamped", got)
	}
	if accumulator != simDT/2 {
		t.Fatalf("accumulator = %v, want %v", accumulator, simDT/2)
	}
}

func TestPlanStepsMaxZeroDoesNotClamp(t *testing.T) {
	const simDT = 1.0 / 60.0
	accumulator := 0.0

	got := planSteps(1, 10*simDT, simDT, &accumulator, 0)
	if got.Steps != 10 || got.Clamped {
		t.Fatalf("plan = %+v, want 10 unclamped", got)
	}
}

func TestPlanStepsIgnoresInvalidTiming(t *testing.T) {
	accumulator := 1.0

	got := planSteps(1, 1, 0, &accumulator, 0)
	if got.Steps != 0 || got.Clamped {
		t.Fatalf("plan = %+v, want 0 unclamped", got)
	}
	if accumulator != 1.0 {
		t.Fatalf("accumulator = %v, want 1", accumulator)
	}
}

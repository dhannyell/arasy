package canvas

import "testing"

func TestControlStateDefaultsToRunningWithSpeedAndDebug(t *testing.T) {
	controls := controlState{
		speed: 1,
		debug: true,
	}

	state := controls.consumeFrameState()

	if state.speed != 1 {
		t.Fatalf("speed = %v, want 1", state.speed)
	}

	if state.paused {
		t.Fatal("paused = true, want false")
	}

	if !state.debug {
		t.Fatal("debug = false, want true")
	}

	if state.steps != 0 {
		t.Fatalf("steps = %v, want 0", state.steps)
	}
}

func TestControlStatePauseAndPlay(t *testing.T) {
	controls := controlState{
		speed: 1,
	}

	controls.apply(clientMessage{
		Action: "pause",
	})

	if state := controls.consumeFrameState(); state.speed != 0 {
		t.Fatalf("paused speed = %v, want 0", state.speed)
	}

	controls.apply(clientMessage{
		Action: "play",
	})

	if state := controls.consumeFrameState(); state.speed != 1 {
		t.Fatalf("playing speed = %v, want 1", state.speed)
	}
}

func TestControlStateStepPausesAndConsumesPendingStep(t *testing.T) {
	controls := controlState{
		speed: 1,
	}

	controls.apply(clientMessage{
		Action: "step",
	})

	state := controls.consumeFrameState()
	if state.speed != 0 {
		t.Fatalf("speed after step = %v, want 0", state.speed)
	}

	if !state.paused {
		t.Fatal("paused after step = false, want true")
	}

	if state.steps != 1 {
		t.Fatalf("steps after step = %v, want 1", state.steps)
	}

	state = controls.consumeFrameState()
	if state.steps != 0 {
		t.Fatalf("steps after consume = %v, want 0", state.steps)
	}
}

func TestControlStateBackPausesAndConsumesPendingBack(t *testing.T) {
	controls := controlState{
		speed: 1,
	}

	controls.apply(clientMessage{
		Action: "back",
	})

	state := controls.consumeFrameState()
	if state.speed != 0 {
		t.Fatalf("speed after back = %v, want 0", state.speed)
	}

	if !state.paused {
		t.Fatal("paused after back = false, want true")
	}

	if state.backs != 1 {
		t.Fatalf("backs after back = %v, want 1", state.backs)
	}

	state = controls.consumeFrameState()
	if state.backs != 0 {
		t.Fatalf("backs after consume = %v, want 0", state.backs)
	}
}

func TestControlStateResetConsumesResetAndPendingNavigation(t *testing.T) {
	controls := controlState{
		speed: 1,
	}

	controls.apply(clientMessage{Action: "step"})
	controls.apply(clientMessage{Action: "back"})
	controls.apply(clientMessage{Action: "reset"})

	state := controls.consumeFrameState()
	if !state.reset {
		t.Fatal("reset = false, want true")
	}

	if state.steps != 0 {
		t.Fatalf("steps after reset = %v, want 0", state.steps)
	}

	if state.backs != 0 {
		t.Fatalf("backs after reset = %v, want 0", state.backs)
	}

	state = controls.consumeFrameState()
	if state.reset {
		t.Fatal("reset after consume = true, want false")
	}
}

func TestControlStateClampsSpeed(t *testing.T) {
	controls := controlState{
		speed: 1,
	}

	controls.apply(clientMessage{
		Action: "setSpeed",
		Speed:  -10,
	})

	if state := controls.consumeFrameState(); state.speed != 1 {
		t.Fatalf("low clamped speed = %v, want 1", state.speed)
	}

	controls.apply(clientMessage{
		Action: "setSpeed",
		Speed:  99,
	})

	if state := controls.consumeFrameState(); state.speed != 8 {
		t.Fatalf("high clamped speed = %v, want 8", state.speed)
	}
}

func TestControlStateTogglesDebug(t *testing.T) {
	controls := controlState{
		speed: 1,
		debug: true,
	}

	controls.apply(clientMessage{
		Action: "setDebug",
		Debug:  false,
	})

	if state := controls.consumeFrameState(); state.debug {
		t.Fatal("debug = true, want false")
	}
}

func TestNextSimulationStepsRunsMultipleTicksWhenSimulationIsFasterThanRender(t *testing.T) {
	var accumulator float64

	steps := nextSimulationSteps(
		0,
		1,
		1.0/60.0,
		1.0/120.0,
		&accumulator,
	)

	if steps != 2 {
		t.Fatalf("steps = %v, want 2", steps)
	}

	if accumulator != 0 {
		t.Fatalf("accumulator = %v, want 0", accumulator)
	}
}

func TestNextSimulationStepsAccumulatesFractionalTicks(t *testing.T) {
	var accumulator float64

	steps := nextSimulationSteps(
		0,
		1,
		1.0/60.0,
		1.0/30.0,
		&accumulator,
	)

	if steps != 0 {
		t.Fatalf("first steps = %v, want 0", steps)
	}

	steps = nextSimulationSteps(
		0,
		1,
		1.0/60.0,
		1.0/30.0,
		&accumulator,
	)

	if steps != 1 {
		t.Fatalf("second steps = %v, want 1", steps)
	}
}

func TestNextSimulationStepsUsesManualStepsWithoutAccumulator(t *testing.T) {
	accumulator := 0.5

	steps := nextSimulationSteps(
		3,
		0,
		1.0/60.0,
		1.0/60.0,
		&accumulator,
	)

	if steps != 3 {
		t.Fatalf("manual steps = %v, want 3", steps)
	}

	if accumulator != 0.5 {
		t.Fatalf("accumulator = %v, want 0.5", accumulator)
	}
}

func TestNextSimulationStepsReturnsZeroWhenPaused(t *testing.T) {
	var accumulator float64

	steps := nextSimulationSteps(
		0,
		0,
		1.0/60.0,
		1.0/60.0,
		&accumulator,
	)

	if steps != 0 {
		t.Fatalf("steps = %v, want 0", steps)
	}
}

func TestHubHistoryLenDefaultsToZero(t *testing.T) {
	h := newHub(&Server{}, 60, 60)

	if h.historyLen() != 0 {
		t.Fatalf("historyLen() = %v, want 0", h.historyLen())
	}
}

func TestHubHistoryLenUsesServerCallback(t *testing.T) {
	h := newHub(&Server{
		HistoryLen: func() int {
			return 12
		},
	}, 60, 60)

	if h.historyLen() != 12 {
		t.Fatalf("historyLen() = %v, want 12", h.historyLen())
	}
}

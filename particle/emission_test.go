package particle

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func TestEmissionStateFirstAdvanceSeedsBurstAndPosition(t *testing.T) {
	var state EmissionState
	pos := math2d.Vec2{X: 4, Y: -2}

	frame := state.Advance(pos, 2, 1, 3, 5, 0.5)

	if frame.Count != 1 {
		t.Fatalf("Count = %d, want 1 continuous particle and no retroactive burst", frame.Count)
	}
	if frame.Base != 0 {
		t.Fatalf("Base = %d, want 0", frame.Base)
	}
	if frame.PrevPos != pos {
		t.Fatalf("PrevPos = %v, want first position %v", frame.PrevPos, pos)
	}
	if state.LastBurst != 5 || state.LastPos != pos || !state.started {
		t.Fatalf("state was not seeded: %+v", state)
	}
}

func TestEmissionStateAccumulatesFractionalRate(t *testing.T) {
	var state EmissionState
	pos := math2d.Vec2{}

	first := state.Advance(pos, 2.5, 1, 0, 0, 1)
	second := state.Advance(pos, 2.5, 1, 0, 0, 1)

	if first.Count != 2 || second.Count != 3 || state.Acc != 0 {
		t.Fatalf("counts = (%d, %d), Acc = %v; want (2, 3), 0", first.Count, second.Count, state.Acc)
	}
}

func TestEmissionStateCountsCumulativeBurstDelta(t *testing.T) {
	var state EmissionState
	pos0 := math2d.Vec2{X: 1}
	pos1 := math2d.Vec2{X: 9}
	state.Advance(pos0, 0, 0, 4, 2, 1)

	frame := state.Advance(pos1, 0, 0, 4, 5, 1)

	if frame.Count != 12 {
		t.Fatalf("Count = %d, want 12", frame.Count)
	}
	if frame.PrevPos != pos0 {
		t.Fatalf("PrevPos = %v, want %v", frame.PrevPos, pos0)
	}
	if state.LastBurst != 5 || state.LastPos != pos1 {
		t.Fatalf("state after burst = %+v", state)
	}
}

func TestEmissionStateNonPositiveDtOnlyTracksPosition(t *testing.T) {
	state := EmissionState{Acc: 0.25, Emitted: 7}
	pos0 := math2d.Vec2{X: 1}
	pausedPos := math2d.Vec2{X: 2}
	state.Advance(pos0, 0, 0, 3, 4, 1)

	frame := state.Advance(pausedPos, 100, 1, 3, 9, 0)

	if frame.Count != 0 || frame.Base != 7 || frame.PrevPos != pos0 {
		t.Fatalf("paused frame = %+v, want Count 0, Base 7, PrevPos %v", frame, pos0)
	}
	if state.Acc != 0.25 || state.LastBurst != 4 || state.LastPos != pausedPos {
		t.Fatalf("paused state = %+v; accounting changed beyond LastPos", state)
	}
}

func TestEmissionStateAdvanceDoesNotConsumeSpawnCounter(t *testing.T) {
	state := EmissionState{Emitted: 41}

	frame := state.Advance(math2d.Vec2{}, 3, 1, 0, 0, 1)

	if frame.Base != 41 || state.Emitted != 41 {
		t.Fatalf("Base = %d, Emitted = %d; want both 41", frame.Base, state.Emitted)
	}
}

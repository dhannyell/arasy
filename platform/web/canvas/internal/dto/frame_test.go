package dto

import (
	"testing"

	"github.com/dhannyell/frameforge/render"
)

func TestNewFrameDTOWithStateIncludesServerState(t *testing.T) {
	state := StateDTO{
		Paused:     true,
		Speed:      3,
		Debug:      1,
		HistoryLen: 42,
	}

	frame := NewFrameDTOWithState(
		9,
		render.Camera{},
		render.Color{},
		state,
		nil,
	)

	if frame.Type != "frame" {
		t.Fatalf("Type = %q, want frame", frame.Type)
	}

	if frame.Tick != 9 {
		t.Fatalf("Tick = %v, want 9", frame.Tick)
	}

	if frame.State != state {
		t.Fatalf("State = %+v, want %+v", frame.State, state)
	}
}

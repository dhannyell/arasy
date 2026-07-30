package wasm

import (
	"math"
	"testing"
)

func TestPendingInputMillisTracksPolledGamepadEvent(t *testing.T) {
	if got := pendingInputMillis(-1, 12, 16, 2); got != 12 {
		t.Fatalf("pending timestamp = %v, want 12", got)
	}
}

func TestPendingInputMillisIgnoresSilentPoll(t *testing.T) {
	if got := pendingInputMillis(-1, 12, 16, 0); got != -1 {
		t.Fatalf("pending timestamp = %v, want unchanged -1", got)
	}
}

func TestPendingInputMillisKeepsOldestTimestamp(t *testing.T) {
	if got := pendingInputMillis(8, 12, 16, 1); got != 8 {
		t.Fatalf("pending timestamp = %v, want oldest 8", got)
	}
	if got := pendingInputMillis(12, 8, 16, 1); got != 8 {
		t.Fatalf("pending timestamp = %v, want older candidate 8", got)
	}
}

func TestPendingInputMillisRejectsInvalidTimestamp(t *testing.T) {
	for _, candidate := range []float64{-1, 17, math.NaN()} {
		if got := pendingInputMillis(8, candidate, 16, 1); got != 8 {
			t.Fatalf("pending timestamp for %v = %v, want unchanged 8", candidate, got)
		}
	}
}

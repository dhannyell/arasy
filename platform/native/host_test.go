//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"testing"
	"time"

	"github.com/dhannyell/arasy/input"
)

func TestFrameBudget(t *testing.T) {
	tests := []struct {
		fps  int
		want time.Duration
	}{
		{60, time.Second / 60},
		{120, time.Second / 120},
		{0, 0},
		{-1, 0},
	}
	for _, tt := range tests {
		got := frameBudget(tt.fps)
		if got != tt.want {
			t.Errorf("frameBudget(%d) = %v; want %v", tt.fps, got, tt.want)
		}
	}
}

func TestPacerBudgetStaysOffWhenVSyncPaces(t *testing.T) {
	if got := pacerBudget(60, VSyncOn); got != 0 {
		t.Fatalf("pacerBudget(60, VSyncOn) = %v, want 0: the swapchain is the clock", got)
	}
	if got := pacerBudget(60, VSyncOff); got != time.Second/60 {
		t.Fatalf("pacerBudget(60, VSyncOff) = %v, want %v", got, time.Second/60)
	}
	if got := pacerBudget(0, VSyncOff); got != 0 {
		t.Fatalf("pacerBudget(0, VSyncOff) = %v, want uncapped", got)
	}
}

func TestPointerMovesCoalesceWithoutCrossingOtherInput(t *testing.T) {
	h := &host{}
	h.queuePointerMove(1, 2)
	h.queuePointerMove(3, 4)
	if len(h.pending) != 1 || h.pending[0].X != 3 || h.pending[0].Y != 4 {
		t.Fatalf("coalesced moves = %+v", h.pending)
	}

	h.pending = append(h.pending, input.Event{Kind: input.EventButtonDown})
	h.queuePointerMove(5, 6)
	if len(h.pending) != 3 || h.pending[2].X != 5 || h.pending[2].Y != 6 {
		t.Fatalf("move crossed intervening input: %+v", h.pending)
	}
}

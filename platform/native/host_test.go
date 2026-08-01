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

func TestCPUFrameTimingUsesSubmitBoundaryAndExcludesAcquireWait(t *testing.T) {
	base := time.Unix(100, 0)
	tickEnd := base.Add(2 * time.Millisecond)
	inputAt := base.Add(-3 * time.Millisecond)
	timing := cpuFrameTiming{
		submittedAt: base.Add(8 * time.Millisecond),
		cpuEndAt:    base.Add(10 * time.Millisecond),
		acquireWait: 4 * time.Millisecond,
		valid:       true,
	}

	if got, want := timing.renderDurationSince(tickEnd), 4*time.Millisecond; got != want {
		t.Fatalf("CPU render duration = %v, want %v", got, want)
	}
	latency, ok := timing.inputToSubmit(inputAt)
	if !ok {
		t.Fatal("input-to-submit timing unavailable")
	}
	if want := 11 * time.Millisecond; latency != want {
		t.Fatalf("input-to-submit = %v, want %v", latency, want)
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

//go:build windows && amd64

package native

import (
	"testing"
	"time"
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

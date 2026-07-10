package runtime_test

import (
	"testing"
	"time"

	"github.com/dhannyell/frameforge/runtime"
)

func TestFrameTimerFirstFrameAndReset(t *testing.T) {
	var timer runtime.FrameTimer
	first := time.Unix(100, 0)
	if got := timer.Elapsed(first); got != 0 {
		t.Fatalf("first elapsed = %v, want 0", got)
	}
	if got := timer.Elapsed(first.Add(25 * time.Millisecond)); got != 25*time.Millisecond {
		t.Fatalf("second elapsed = %v, want 25ms", got)
	}
	timer.Reset()
	if got := timer.Elapsed(first.Add(time.Second)); got != 0 {
		t.Fatalf("elapsed after reset = %v, want 0", got)
	}
}

func TestFrameTimerElapsedMillis(t *testing.T) {
	var timer runtime.FrameTimer
	if got := timer.ElapsedMillis(1000); got != 0 {
		t.Fatalf("first millisecond elapsed = %v, want 0", got)
	}
	if got := timer.ElapsedMillis(1016.5); got != 16*time.Millisecond+500*time.Microsecond {
		t.Fatalf("millisecond elapsed = %v, want 16.5ms", got)
	}
}

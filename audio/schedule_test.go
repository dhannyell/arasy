package audio

import (
	"math"
	"testing"
)

func TestScheduledTrackStartUsesRemainingSimulationTicks(t *testing.T) {
	tests := []struct {
		name        string
		currentTick uint64
		alpha       float64
		startTick   uint64
		want        float64
	}{
		{name: "immediate zero value", currentTick: 6, startTick: 0, want: 10},
		{name: "future tick", currentTick: 6, alpha: 0.5, startTick: 10, want: 11.75},
		{name: "past tick", currentTick: 12, alpha: 0.5, startTick: 10, want: 8.75},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ScheduledTrackStart(10, test.currentTick, test.alpha, test.startTick, 0.5); got != test.want {
				t.Fatalf("scheduledTrackStart() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestScheduledPlaybackUsesAudioClockAndCompensatesLateDecode(t *testing.T) {
	tests := []struct {
		name        string
		now         float64
		scheduledAt float64
		duration    float64
		loop        bool
		wantStart   float64
		wantOffset  float64
		wantExpired bool
	}{
		{name: "future", now: 10, scheduledAt: 12, duration: 30, wantStart: 12},
		{name: "late decode", now: 12.25, scheduledAt: 12, duration: 30, wantStart: 12.25, wantOffset: 0.25},
		{name: "expired one shot", now: 43, scheduledAt: 12, duration: 30, wantStart: 43, wantExpired: true},
		{name: "late loop wraps", now: 43.25, scheduledAt: 12, duration: 10, loop: true, wantStart: 43.25, wantOffset: 1.25},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, offset, expired := ScheduledPlayback(test.now, test.scheduledAt, test.duration, test.loop)
			if start != test.wantStart || offset != test.wantOffset || expired != test.wantExpired {
				t.Fatalf("scheduledPlayback() = (%v, %v, %t), want (%v, %v, %t)", start, offset, expired, test.wantStart, test.wantOffset, test.wantExpired)
			}
		})
	}
}

func TestExpectedPlayhead(t *testing.T) {
	tests := []struct {
		name        string
		currentTick float64
		alpha       float64
		startTick   uint64
		duration    float64
		loop        bool
		wantOffset  float64
		wantState   PlayheadState
	}{
		{name: "unanchored", currentTick: 10, startTick: 0, duration: 3, wantState: PlayheadUnanchored},
		{name: "pending", currentTick: 9, alpha: 0.5, startTick: 10, duration: 3, wantState: PlayheadPending},
		{name: "playing", currentTick: 12, alpha: 0.5, startTick: 10, duration: 3, wantOffset: 1.25, wantState: PlayheadPlaying},
		{name: "finished one shot", currentTick: 16, startTick: 10, duration: 3, wantState: PlayheadFinished},
		{name: "loop wraps", currentTick: 25, startTick: 10, duration: 3, loop: true, wantOffset: 1.5, wantState: PlayheadPlaying},
		{name: "unknown duration keeps raw offset", currentTick: 25, startTick: 10, wantOffset: 7.5, wantState: PlayheadPlaying},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			offset, state := ExpectedPlayhead(test.currentTick, test.alpha, test.startTick, 0.5, test.duration, test.loop)
			if offset != test.wantOffset || state != test.wantState {
				t.Fatalf("ExpectedPlayhead() = (%v, %v), want (%v, %v)", offset, state, test.wantOffset, test.wantState)
			}
		})
	}
}

func TestExpectedTrackPlayheadSubtractsPausedTicks(t *testing.T) {
	track := Track{StartTick: 10, PausedTicks: 2}

	offset, state := ExpectedTrackPlayhead(20, 0, track, 0.5, 30)
	if offset != 4 || state != PlayheadPlaying {
		t.Fatalf("resumed ExpectedTrackPlayhead() = (%v, %v), want (4, %v)", offset, state, PlayheadPlaying)
	}

	track.Paused = true
	track.PausedAtTick = 15
	offset, state = ExpectedTrackPlayhead(20, 0, track, 0.5, 30)
	if offset != 1.5 || state != PlayheadPlaying {
		t.Fatalf("paused ExpectedTrackPlayhead() = (%v, %v), want (1.5, %v)", offset, state, PlayheadPlaying)
	}
}

func TestPlayheadDistance(t *testing.T) {
	tests := []struct {
		name     string
		a, b     float64
		duration float64
		loop     bool
		want     float64
	}{
		{name: "linear", a: 3.25, b: 1, want: 2.25},
		{name: "circular loop boundary", a: 0.01, b: 9.99, duration: 10, loop: true, want: 0.02},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := PlayheadDistance(test.a, test.b, test.duration, test.loop); math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("PlayheadDistance() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNeedsResync(t *testing.T) {
	if NeedsResync(DefaultResyncToleranceSec, 0, 0, DefaultResyncToleranceSec, false) {
		t.Fatal("NeedsResync() = true at the tolerance boundary, want false")
	}
	if !NeedsResync(DefaultResyncToleranceSec+0.001, 0, 0, DefaultResyncToleranceSec, false) {
		t.Fatal("NeedsResync() = false above the tolerance boundary, want true")
	}
}

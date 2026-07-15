package wasm

import "testing"

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
			if got := scheduledTrackStart(10, test.currentTick, test.alpha, test.startTick, 0.5); got != test.want {
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
			start, offset, expired := scheduledPlayback(test.now, test.scheduledAt, test.duration, test.loop)
			if start != test.wantStart || offset != test.wantOffset || expired != test.wantExpired {
				t.Fatalf("scheduledPlayback() = (%v, %v, %t), want (%v, %v, %t)", start, offset, expired, test.wantStart, test.wantOffset, test.wantExpired)
			}
		})
	}
}

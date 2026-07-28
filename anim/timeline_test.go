package anim_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/ease"
)

func TestTimelineTrackWindows(t *testing.T) {
	tests := []struct {
		name    string
		elapsed float64
		start   float64
		end     float64
		want    float64
		done    bool
	}{
		{name: "before", elapsed: 0, start: 0.5, end: 1, want: 0},
		{name: "inside", elapsed: 0.75, start: 0.5, end: 1, want: 0.5},
		{name: "at end", elapsed: 1, start: 0.5, end: 1, want: 1, done: true},
		{name: "after", elapsed: 2, start: 0.5, end: 1, want: 1, done: true},
		{name: "zero window", elapsed: 0, start: 1, end: 1, want: 1, done: true},
		{name: "negative window", elapsed: 0, start: 2, end: 1, want: 1, done: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			track := anim.At(test.elapsed).Track(test.start, test.end, ease.Linear)
			if math.Abs(track.Value()-test.want) > 1e-9 || track.Done() != test.done {
				t.Fatalf("track = (%v, %v), want (%v, %v)", track.Value(), track.Done(), test.want, test.done)
			}
		})
	}
}

func TestInactiveTimeline(t *testing.T) {
	timeline := anim.Inactive()
	if !timeline.Done(100) {
		t.Fatal("inactive timeline must be done")
	}
	track := timeline.Track(0, 1, ease.Linear)
	if track.Value() != 0 || track.Done() {
		t.Fatalf("inactive track = (%v, %v), want zero value", track.Value(), track.Done())
	}
}

func TestTimelineElapsedAndLerp(t *testing.T) {
	timeline := anim.At(0.5)
	if timeline.Elapsed() != 0.5 {
		t.Fatalf("Elapsed() = %v, want 0.5", timeline.Elapsed())
	}
	if got := timeline.Track(0, 1, ease.Linear).Lerp(10, 20); got != 15 {
		t.Fatalf("Lerp = %v, want 15", got)
	}
}

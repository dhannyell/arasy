package loop_test

import (
	"testing"
	"time"

	"github.com/dhannyell/frameforge/loop"
)

func TestNewFixedStep(t *testing.T) {
	fixed := loop.NewFixedStep(time.Second / 60)

	if fixed.Step() != time.Second/60 {
		t.Fatalf("got %v, want %v", fixed.Step(), time.Second/60)
	}
}

func TestFixedStepShouldUpdate(t *testing.T) {
	tests := []struct {
		name    string
		step    time.Duration
		elapsed time.Duration
		want    bool
	}{
		{
			name:    "less than one step",
			step:    16 * time.Millisecond,
			elapsed: 15 * time.Millisecond,
			want:    false,
		},
		{
			name:    "exactly one step",
			step:    16 * time.Millisecond,
			elapsed: 16 * time.Millisecond,
			want:    true,
		},
		{
			name:    "more than one step",
			step:    16 * time.Millisecond,
			elapsed: 30 * time.Millisecond,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := loop.NewFixedStep(tt.step)
			fixed.Add(tt.elapsed)

			got := fixed.ShouldUpdate()

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFixedStepConsume(t *testing.T) {
	fixed := loop.NewFixedStep(10 * time.Millisecond)
	fixed.Add(25 * time.Millisecond)

	if !fixed.ShouldUpdate() {
		t.Fatal("expected enough accumulated time for first update")
	}

	fixed.Consume()

	if !fixed.ShouldUpdate() {
		t.Fatal("expected enough accumulated time for second update")
	}

	fixed.Consume()

	if fixed.ShouldUpdate() {
		t.Fatal("did not expect enough accumulated time for third update")
	}
}

func TestFixedStepAlpha(t *testing.T) {
	tests := []struct {
		name    string
		step    time.Duration
		elapsed time.Duration
		want    float64
	}{
		{
			name:    "empty accumulator",
			step:    10 * time.Millisecond,
			elapsed: 0,
			want:    0,
		},
		{
			name:    "half step",
			step:    10 * time.Millisecond,
			elapsed: 5 * time.Millisecond,
			want:    0.5,
		},
		{
			name:    "one quarter step",
			step:    20 * time.Millisecond,
			elapsed: 5 * time.Millisecond,
			want:    0.25,
		},
		{
			name:    "zero step",
			step:    0,
			elapsed: 5 * time.Millisecond,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := loop.NewFixedStep(tt.step)
			fixed.Add(tt.elapsed)

			got := fixed.Alpha()

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFixedStepAlphaAfterConsume(t *testing.T) {
	fixed := loop.NewFixedStep(10 * time.Millisecond)
	fixed.Add(25 * time.Millisecond)

	fixed.Consume()
	fixed.Consume()

	got := fixed.Alpha()
	want := 0.5

	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

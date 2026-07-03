package loop

import (
	"time"
)

// FixedStep accumulates elapsed time for fixed updates.
type FixedStep struct {
	step        time.Duration
	accumulator time.Duration
}

// NewFixedStep creates a fixed timestep accumulator.
func NewFixedStep(step time.Duration) *FixedStep {
	return &FixedStep{step: step}
}

// Step returns the configured fixed update duration.
func (f *FixedStep) Step() time.Duration {
	return f.step
}

// Add adds elapsed time to the accumulator.
func (f *FixedStep) Add(elapsed time.Duration) {
	f.accumulator += elapsed
}

// ShouldUpdate reports whether there is enough accumulated time for one update.
func (f *FixedStep) ShouldUpdate() bool {
	return f.accumulator >= f.step
}

// Consume removes one fixed step from the accumulator.
func (f *FixedStep) Consume() {
	f.accumulator -= f.step
}

// Alpha returns the interpolation factor between the previous and the next state.
func (f *FixedStep) Alpha() float64 {
	if f.step == 0 {
		return 0
	}

	return float64(f.accumulator) / float64(f.step)
}

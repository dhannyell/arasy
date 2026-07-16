package particle

import "math"

const (
	budgetEMAAlpha = 0.10
	budgetDecrease = 0.85
	budgetRecovery = 0.04
)

// Budget adaptively limits the total number of live particles across all
// emitters on CPU backends. Additive-increase/multiplicative-decrease on
// the smoothed frame delta keeps the frame time near target without
// oscillating. GPU backends do not use it.
type Budget struct {
	target  float64
	floor   int
	ceiling int

	cap            float64
	ema            float64
	hasObservation bool
}

// NewBudget creates a budget that starts at its maximum capacity.
//
// Target is the desired render-frame duration in seconds. Floor and ceiling
// are global particle counts, not per-emitter limits.
func NewBudget(target float64, floor, ceiling int) *Budget {
	if math.IsNaN(target) || math.IsInf(target, 0) || target <= 0 {
		panic("particle: budget target must be finite and positive")
	}
	if floor < 0 {
		panic("particle: budget floor must not be negative")
	}
	if ceiling < floor {
		panic("particle: budget ceiling must not be below floor")
	}

	return &Budget{
		target:  target,
		floor:   floor,
		ceiling: ceiling,
		cap:     float64(ceiling),
	}
}

// SetCeiling updates the maximum global particle count without resetting the
// frame-time EMA or an adaptive reduction already in effect.
//
// Raising an unconstrained budget (Cap == the old ceiling) opens the new
// capacity immediately. Raising a budget that was reduced under pressure
// preserves its current absolute cap; subsequent observations recover it
// gradually. Lowering the ceiling clamps the cap to the new range.
func (b *Budget) SetCeiling(ceiling int) {
	if ceiling < b.floor {
		panic("particle: budget ceiling must not be below floor")
	}

	wasAtCeiling := b.cap >= float64(b.ceiling)
	b.ceiling = ceiling

	if wasAtCeiling {
		b.cap = float64(ceiling)
	}
	b.cap = max(float64(b.floor), min(b.cap, float64(ceiling)))
}

// Observe records one render-frame duration in seconds.
//
// It must be called exactly once per rendered frame, not once per emitter.
// Non-positive and non-finite samples are ignored.
func (b *Budget) Observe(dt float64) {
	if dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}

	if !b.hasObservation {
		b.ema = dt
		b.hasObservation = true
	} else {
		b.ema += budgetEMAAlpha * (dt - b.ema)
	}

	switch {
	case b.ema > b.target:
		b.cap *= budgetDecrease
	case b.ema <= b.target*0.8:
		b.cap += float64(b.ceiling) * budgetRecovery * dt
	}

	b.cap = max(float64(b.floor), min(b.cap, float64(b.ceiling)))
}

// Cap returns the current global live-particle limit.
func (b *Budget) Cap() int {
	return int(b.cap)
}

// Share returns this emitter's proportional slice of the global cap.
//
// Want is this emitter's requested capacity. TotalWant is the sum of all
// participating emitter capacities for the current frame.
func (b *Budget) Share(want, totalWant int) int {
	if want <= 0 || totalWant <= 0 {
		return 0
	}

	want = min(want, totalWant)

	capacity := b.Cap()
	if capacity >= totalWant {
		return want
	}

	share := int(
		float64(want) *
			float64(capacity) /
			float64(totalWant),
	)

	return min(share, want)
}

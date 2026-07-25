package particle_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/particle"
)

func TestNewBudgetStartsAtCeiling(t *testing.T) {
	b := particle.NewBudget(0.02, 100, 1000)

	if got := b.Cap(); got != 1000 {
		t.Fatalf("Cap() = %d, want 1000 (ceiling)", got)
	}
}

func TestNewBudgetPanicsOnInvalidParameters(t *testing.T) {
	tests := []struct {
		name           string
		target         float64
		floor, ceiling int
	}{
		{name: "zero target", target: 0, floor: 0, ceiling: 100},
		{name: "negative target", target: -0.02, floor: 0, ceiling: 100},
		{name: "NaN target", target: math.NaN(), floor: 0, ceiling: 100},
		{name: "infinite target", target: math.Inf(1), floor: 0, ceiling: 100},
		{name: "negative floor", target: 0.02, floor: -1, ceiling: 100},
		{name: "ceiling below floor", target: 0.02, floor: 200, ceiling: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewBudget(%v, %d, %d) did not panic", tt.target, tt.floor, tt.ceiling)
				}
			}()
			particle.NewBudget(tt.target, tt.floor, tt.ceiling)
		})
	}
}

func TestBudgetSetCeiling(t *testing.T) {
	t.Run("raising unconstrained budget opens new capacity", func(t *testing.T) {
		b := particle.NewBudget(0.02, 100, 1000)

		b.SetCeiling(2000)

		if got := b.Cap(); got != 2000 {
			t.Fatalf("Cap() after raising ceiling = %d, want 2000", got)
		}
	})

	t.Run("raising constrained budget preserves cap and EMA", func(t *testing.T) {
		b := particle.NewBudget(0.02, 100, 1000)
		b.Observe(0.04) // cap 850; EMA 40 ms, above target

		b.SetCeiling(2000)
		if got := b.Cap(); got != 850 {
			t.Fatalf("Cap() after raising constrained ceiling = %d, want 850", got)
		}

		b.Observe(0.001)
		if got := b.Cap(); got != 722 {
			t.Fatalf("Cap() after retained high EMA observation = %d, want 722", got)
		}
	})

	t.Run("lowering ceiling clamps cap", func(t *testing.T) {
		b := particle.NewBudget(0.02, 100, 1000)

		b.SetCeiling(400)

		if got := b.Cap(); got != 400 {
			t.Fatalf("Cap() after lowering ceiling = %d, want 400", got)
		}
	})

	t.Run("ceiling below floor panics", func(t *testing.T) {
		b := particle.NewBudget(0.02, 100, 1000)
		defer func() {
			if recover() == nil {
				t.Fatal("SetCeiling below floor did not panic")
			}
		}()

		b.SetCeiling(99)
	})
}

func TestBudgetObserveIgnoresNonPositiveAndNonFiniteSamples(t *testing.T) {
	tests := []struct {
		name string
		dt   float64
	}{
		{name: "zero", dt: 0},
		{name: "negative", dt: -1},
		{name: "NaN", dt: math.NaN()},
		{name: "positive infinity", dt: math.Inf(1)},
		{name: "negative infinity", dt: math.Inf(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := particle.NewBudget(0.02, 100, 1000)

			b.Observe(tt.dt)

			if got := b.Cap(); got != 1000 {
				t.Fatalf("Cap() after Observe(%v) = %d, want unchanged 1000", tt.dt, got)
			}
		})
	}
}

func TestBudgetFirstObserveSeedsEMAAndAppliesDecreaseImmediately(t *testing.T) {
	// dt (0.03) is above target (0.02), so the very first observation both
	// seeds the EMA and applies the multiplicative decrease in the same
	// call - there is no "warm-up" observation that skips adjustment.
	b := particle.NewBudget(0.02, 10, 1000)

	b.Observe(0.03)

	if got := b.Cap(); got != 850 {
		t.Fatalf("Cap() after first Observe(0.03) = %d, want 850 (1000 * 0.85)", got)
	}
}

func TestBudgetDecreasesAtMostOncePerObservation(t *testing.T) {
	// Holding dt constant locks the EMA to that value after the first
	// call (delta becomes zero), making each subsequent step a single,
	// predictable *0.85 multiplication - this pins down "at most once per
	// observation" without needing to hand-compute a float chain.
	b := particle.NewBudget(0.02, 1, 1000)

	prev := b.Cap()
	for i := 0; i < 20; i++ {
		b.Observe(0.5) // far above target: always takes the decrease branch
		cur := b.Cap()
		if cur > prev {
			t.Fatalf("Cap() increased under sustained pressure at step %d: %d -> %d", i, prev, cur)
		}
		prev = cur
	}
}

func TestBudgetClampsAtFloorUnderSustainedPressure(t *testing.T) {
	b := particle.NewBudget(0.02, 50, 1000)

	for i := 0; i < 200; i++ {
		b.Observe(0.1) // far above target
	}

	if got := b.Cap(); got != 50 {
		t.Fatalf("Cap() after sustained pressure = %d, want floor 50", got)
	}
}

func TestBudgetRecoversAdditivelyAfterPressureEases(t *testing.T) {
	b := particle.NewBudget(1.0, 100, 1000)

	// Drive the cap down to the floor first.
	for i := 0; i < 50; i++ {
		b.Observe(5.0) // far above target: decrease branch
	}
	reduced := b.Cap()
	if reduced != 100 {
		t.Fatalf("Cap() after pressure = %d, want floor 100", reduced)
	}

	// Ease off: dt well under 0.8*target lets the EMA settle down and the
	// recovery branch (additive, scaled by ceiling*0.04*dt) take over.
	prev := b.Cap()
	sawIncrease := false
	for i := 0; i < 500; i++ {
		b.Observe(0.05)
		cur := b.Cap()
		if cur < prev {
			t.Fatalf("Cap() decreased during recovery at step %d: %d -> %d", i, prev, cur)
		}
		if cur > prev {
			sawIncrease = true
		}
		if cur > 1000 {
			t.Fatalf("Cap() exceeded ceiling during recovery at step %d: %d", i, cur)
		}
		prev = cur
	}

	if !sawIncrease {
		t.Fatal("Cap() never increased while pressure eased, want at least one recovery step")
	}
}

func TestBudgetNeverLeavesFloorCeilingBoundsUnderAlternatingPressure(t *testing.T) {
	b := particle.NewBudget(0.02, 20, 500)

	dts := []float64{0.1, 0.001, 0.05, 0.0005, 0.2, 0.0001}
	for i := 0; i < 300; i++ {
		b.Observe(dts[i%len(dts)])
		if got := b.Cap(); got < 20 || got > 500 {
			t.Fatalf("Cap() at step %d = %d, want within [20, 500]", i, got)
		}
	}
}

func TestBudgetShare(t *testing.T) {
	t.Run("invalid want returns zero", func(t *testing.T) {
		b := particle.NewBudget(0.02, 10, 1000)
		if got := b.Share(0, 100); got != 0 {
			t.Fatalf("Share(0, 100) = %d, want 0", got)
		}
		if got := b.Share(-5, 100); got != 0 {
			t.Fatalf("Share(-5, 100) = %d, want 0", got)
		}
	})

	t.Run("invalid totalWant returns zero", func(t *testing.T) {
		b := particle.NewBudget(0.02, 10, 1000)
		if got := b.Share(50, 0); got != 0 {
			t.Fatalf("Share(50, 0) = %d, want 0", got)
		}
		if got := b.Share(50, -1); got != 0 {
			t.Fatalf("Share(50, -1) = %d, want 0", got)
		}
	})

	t.Run("cap above totalWant returns want unchanged", func(t *testing.T) {
		b := particle.NewBudget(0.02, 10, 1000) // Cap() == 1000

		if got := b.Share(50, 100); got != 50 {
			t.Fatalf("Share(50, 100) = %d, want 50 (cap covers total demand)", got)
		}
	})

	t.Run("want above totalWant is clamped to totalWant first", func(t *testing.T) {
		b := particle.NewBudget(0.02, 10, 1000) // Cap() == 1000

		if got := b.Share(150, 100); got != 100 {
			t.Fatalf("Share(150, 100) = %d, want 100 (want clamped to totalWant)", got)
		}
	})

	t.Run("cap below totalWant scales want proportionally", func(t *testing.T) {
		// floor == ceiling pins Cap() to an exact known value regardless
		// of Observe(), so the proportional arithmetic is exact.
		b := particle.NewBudget(0.02, 200, 200)

		if got := b.Share(100, 400); got != 50 {
			t.Fatalf("Share(100, 400) = %d, want 50 (100 * 200/400)", got)
		}
	})

	t.Run("proportional share never exceeds want", func(t *testing.T) {
		b := particle.NewBudget(0.02, 200, 200) // Cap() == 200

		if got := b.Share(400, 400); got != 200 {
			t.Fatalf("Share(400, 400) = %d, want 200 (== cap, one emitter owns all demand)", got)
		}
	})
}

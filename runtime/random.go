package runtime

import "math/bits"

// Rand is a deterministic random generator (splitmix64). It is a pure
// value type (state = one uint64): copying the struct IS the snapshot —
// the same discipline as StateSlot. The sequence for a given seed is a
// project-level contract: replays recorded today must reproduce on any
// future Go version, so this generator never changes behavior.
type Rand struct {
	state uint64
}

// NewRand creates a generator from a seed. Same seed ⇒ same sequence,
// forever. Zero is a valid, deterministic seed.
func NewRand(seed uint64) Rand {
	return Rand{state: seed}
}

// Uint64 advances the stream. Use splitmix64 algorithm.
func (r *Rand) Uint64() uint64 {
	// magic initial state. this is hexadecimal golden ratio
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9 // first mixing constant
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB // second mixing constant
	return z ^ (z >> 31)
}

// Float64 returns a uniform value in [0, 1).
func (r *Rand) Float64() float64 {
	return float64(r.Uint64()>>11) / (1 << 53)
}

// Uint64N returns a uniform value in [0, n) without modulo bias, using
// Lemire's debiased multiply-shift.
// Chosen NOW because the sequence freezes forever at the first saved
// replay — switching from a biased modulo later would break replays.
// It may consume more than one Uint64 draw (rejection loop); that is
// deterministic for a given state and therefore replay-safe.
func (r *Rand) Uint64N(n uint64) uint64 {
	if n == 0 {
		panic("runtime: Uint64N called with n == 0")
	}
	hi, lo := bits.Mul64(r.Uint64(), n)
	if lo < n {
		thresh := -n % n
		for lo < thresh {
			hi, lo = bits.Mul64(r.Uint64(), n)
		}
	}
	return hi
}

// IntN returns a uniform value in [0, n). Panics if n <= 0.
func (r *Rand) IntN(n int) int {
	if n <= 0 {
		panic("runtime: IntN called with n <= 0")
	}
	return int(r.Uint64N(uint64(n)))
}

// Derive returns a deterministic child generator derived from the current
// state and a label. It does NOT advance the parent stream. Consumers with
// multiple streams (e.g. perception/execution/events) derive one child per
// label and store them as values in their own state.
func (r Rand) Derive(label string) Rand {
	h := uint64(14695981039346656037) // FNV-1a 64 offset basis
	for i := 0; i < len(label); i++ {
		h ^= uint64(label[i])
		h *= 1099511628211 // FNV-1a 64 prime
	}
	child := Rand{state: r.state ^ h}
	child.Uint64() // one scramble step so child ≠ parent even for degenerate h
	return child
}

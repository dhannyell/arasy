package simgame

import (
	crand "crypto/rand" // want `crypto/rand bypasses the recorded random stream.*Use ctx\.Rand`
	"math/rand"         // want `math/rand bypasses the recorded random stream.*Use ctx\.Rand`
	"time"
)

// scores stands in for keyed simulation state.
var scores = map[int]float64{}

// Total is order-sensitive: float accumulation depends on visit order.
func Total() float64 {
	var sum float64
	for _, v := range scores { // want `map iteration order is deliberately random.*arasy:order-insensitive`
		sum += v
	}
	return sum
}

// Reset is order-independent, and the directive above the loop says why.
func Reset() {
	//arasy:order-insensitive: pure membership work, no order observed
	for k := range scores {
		delete(scores, k)
	}
}

// OrderedTotal iterates a slice of keys, so it reports nothing.
func OrderedTotal(keys []int) float64 {
	var sum float64
	for _, k := range keys {
		sum += scores[k]
	}
	return sum
}

// Roll draws from a bypassing source; the import line carries the diagnostic.
func Roll() int { return rand.Int() }

// Fill draws from crypto/rand; the import line carries the diagnostic.
func Fill(b []byte) { _, _ = crand.Read(b) }

// Stamp reads the wall clock.
func Stamp() time.Time {
	return time.Now() // want `time\.Now reads the wall clock.*ctx\.Time`
}

// TickBudget uses a Duration constant, which is legitimate: only the three
// clock reads are forbidden, never the time package itself.
const TickBudget = 16 * time.Millisecond

package utilityai_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/utilityai"
)

// TestZeroAllocs asserts the budget the batch design promises: with a nil
// trace, EvaluateBatch works entirely in caller buffers.
func TestZeroAllocs(t *testing.T) {
	p, inputs, prev := batchFixture(1024)
	out := make([]utilityai.OptionID, len(prev))
	allocs := testing.AllocsPerRun(10, func() {
		p.EvaluateBatch(inputs, prev, out, nil)
	})
	if allocs != 0 {
		t.Fatalf("EvaluateBatch without trace allocated %.0f times per run", allocs)
	}
}

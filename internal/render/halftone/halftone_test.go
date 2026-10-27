package halftone

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/render"
)

func TestThresholdsCoverTheRangeEvenly(t *testing.T) {
	for _, shape := range []render.HalftoneShape{render.HalftoneRound, render.HalftoneCross, render.HalftonePlus} {
		for n := 1; n <= MaxCell; n++ {
			got := Thresholds(shape, n)
			seen := make([]bool, n*n)
			for _, m := range got {
				rank := int(math.Round(float64(m)*float64(n*n) - 0.5))
				if rank < 0 || rank >= n*n || seen[rank] {
					t.Fatalf("shape %d n %d: threshold %v is not a distinct rank", shape, n, m)
				}
				seen[rank] = true
			}
		}
	}
}

func TestCrossGrowsFromTheDiagonals(t *testing.T) {
	got := Thresholds(render.HalftoneCross, 4)
	for _, i := range []int{0, 5, 10, 15, 3, 6, 9, 12} {
		if got[i] > 0.5 {
			t.Fatalf("diagonal pixel %d has threshold %v", i, got[i])
		}
	}
}

// TestPlusOpensTwoCrossesPerCell checks that the first ten ranks of a 4x4
// cell are the two five-pixel + shapes at (1,1) and (3,3), arms wrapping.
func TestPlusOpensTwoCrossesPerCell(t *testing.T) {
	got := Thresholds(render.HalftonePlus, 4)
	plus := map[int]bool{5: true, 1: true, 9: true, 4: true, 6: true, 15: true, 11: true, 3: true, 14: true, 12: true}
	for i, m := range got {
		if inPlus := m < 10.0/16; inPlus != plus[i] {
			t.Fatalf("pixel %d threshold %v, want in plus %v", i, m, plus[i])
		}
	}
}

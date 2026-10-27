// Package halftone builds the ordered-dither threshold table the Halftone
// post effect shares between backends.
package halftone

import (
	"math"
	"sort"

	"github.com/dhannyell/arasy/render"
)

// MaxCell is the widest cell the table covers; the GPU uniform holds
// MaxCell*MaxCell thresholds.
const MaxCell = 16

// Cell is the whole-pixel pitch a CellSize resolves to.
func Cell(size float64) int {
	if size <= 0 {
		return 8
	}
	return max(1, min(MaxCell, int(math.Round(size))))
}

// NoiseTick quantises the render clock to the noise redraw rate and folds
// it into the seed, so the noise is a pure function of (x, y, seed+tick).
func NoiseTick(seed uint32, rate, time float64) uint32 {
	if rate <= 0 {
		rate = 12
	}
	return seed + uint32(time*rate)
}

// Pixel is the whole screen pixels one dither pixel covers.
func Pixel(size float64) int {
	return max(1, int(math.Round(size)))
}

// Thresholds ranks the n*n pixels of one cell by how far the shape's dot
// grows before it reaches them, and returns each pixel's threshold as
// (rank+0.5)/n*n in row-major order. Every cell then spends its n*n
// pixels evenly across the tone range whatever n is; sampling a
// continuous distance at pixel centres never reaches the extremes and made
// the response depend on the parity of n.
func Thresholds(shape render.HalftoneShape, n int) []float32 {
	count := n * n
	order := make([]int, count)
	dist := make([]float64, count)
	for i := range count {
		order[i] = i
		if shape == render.HalftonePlus {
			dist[i] = plusDistance(i%n, i/n, n)
			continue
		}
		u := (float64(i%n) + 0.5) / float64(n)
		v := (float64(i/n) + 0.5) / float64(n)
		dist[i] = distance(shape, u, v)
	}
	sort.SliceStable(order, func(a, b int) bool { return dist[order[a]] < dist[order[b]] })
	out := make([]float32, count)
	for rank, i := range order {
		out[i] = float32((float64(rank) + 0.5) / float64(count))
	}
	return out
}

// plusDistance ranks a pixel against two dot centres, at (n/4, n/4) and
// (3n/4, 3n/4) of the cell, with the cell wrapping so the lattice tiles.
// On-axis pixels come first, nearest the centre first, then the rest by how
// far they sit from the nearest axis: the dot opens as a + and only then
// fills its quadrants.
func plusDistance(x, y, n int) float64 {
	best := math.Inf(1)
	for _, c := range [2]int{n / 4, 3 * n / 4} {
		dx := wrapDistance(x-c, n)
		dy := wrapDistance(y-c, n)
		d := float64(min(dx, dy)*n + max(dx, dy))
		best = math.Min(best, d)
	}
	return best
}

func wrapDistance(d, n int) int {
	d = ((d % n) + n) % n
	return min(d, n-d)
}

func distance(shape render.HalftoneShape, u, v float64) float64 {
	if shape == render.HalftoneCross {
		// Nearer of the two diagonals: the dot opens as an X and thickens.
		return math.Min(math.Abs(u-v), math.Abs(u+v-1))
	}
	return math.Hypot(u-0.5, v-0.5)
}

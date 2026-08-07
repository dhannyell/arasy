//arasy:render

// Package rendergame imports the simulation root but opts out with the render
// directive, so the analyzer reports no diagnostics.
package rendergame

import (
	"math"

	"simroot"
)

// Wobble represents a presentation-only offset.
func Wobble(t float64) float64 {
	simroot.Tick()
	return math.Sin(t * 2)
}

//arasy:render

// Package rendergame imports the simulation root but opts out with the render
// directive, so the analyzer reports no diagnostics.
package rendergame

import (
	"math"
	"time"

	"simroot"
)

// Wobble represents a presentation-only offset.
func Wobble(t float64) float64 {
	simroot.Tick()
	return math.Sin(t * 2)
}

// Overlay iterates a presentation cache and reads the wall clock; the file
// directive exempts every hazard here, not only the math one.
func Overlay(cache map[int]float64) float64 {
	var sum float64
	for _, v := range cache {
		sum += v
	}
	return sum + float64(time.Now().Unix())
}

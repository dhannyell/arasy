// Package simgame reaches the simulation root through middle and still gets
// the diagnostic.
package simgame

import (
	"math"

	"middle"
)

// Update represents simulation code.
func Update(x float64) float64 {
	middle.Step()
	return math.Sin(x) // want `math\.Sin is not reproducible across architectures.*Use dmath\.Sin instead.*//arasy:render`
}

// Uncovered calls a forbidden function without a dmath replacement, so the
// diagnostic points to the primitive operations.
func Uncovered(x float64) float64 {
	return math.Tan(x) // want `math\.Tan is not reproducible across architectures.*Build it from the dmath primitives`
}

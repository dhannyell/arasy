// Package pure does not import the simulation root, so its math calls are
// outside the analyzer's scope.
package pure

import "math"

// Halve is outside the simulation path and may use math.
func Halve(x float64) float64 {
	return math.Exp(x) / 2
}

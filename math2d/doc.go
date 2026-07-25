// Package math2d provides basic 2D math primitives used by Arasy.
//
// It contains small, dependency-free types for positions, directions,
// distances and rectangular bounds. These primitives are shared by rendering,
// physics, input handling and simulation code.
//
// The package is intentionally minimal. It favors simple value types and
// explicit operations over a large general-purpose math API.
//
// Basic usage:
//
//	position := math2d.Vec2{X: 10, Y: 20}
//	velocity := math2d.Vec2{X: 2, Y: -1}
//
//	next := position.Add(velocity)
//
// Coordinates in Arasy are represented with float64 values. Individual
// systems may define their own coordinate spaces, such as screen pixels,
// world units or normalized field coordinates.
package math2d

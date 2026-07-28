// Package vector builds immutable paths and tessellates them into render
// commands without introducing path semantics into platform backends.
package vector

import "errors"

var (
	ErrNonFinite          = errors.New("vector: path contains a non-finite coordinate")
	ErrNoCurrentSubpath   = errors.New("vector: operation requires a current subpath")
	ErrEmptyPath          = errors.New("vector: path is empty")
	ErrInvalidTolerance   = errors.New("vector: tolerance must be finite and greater than zero")
	ErrFlattenDepth       = errors.New("vector: curve did not converge within the flattening depth limit")
	ErrFillContour        = errors.New("vector: fill requires closed contours")
	ErrSelfIntersection   = errors.New("vector: self-intersecting contours are not supported")
	ErrDegenerateGeometry = errors.New("vector: geometry is degenerate")
	ErrInvalidStroke      = errors.New("vector: stroke style is invalid")
	ErrInvalidProgress    = errors.New("vector: progress must be finite and in [0,1]")
)

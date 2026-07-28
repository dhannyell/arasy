package render

import "errors"

var (
	ErrTriangleIndexCount = errors.New("render: triangle index count must be a multiple of three")
	ErrTriangleIndexRange = errors.New("render: triangle index is outside the vertex slice")
)

// ValidateTriangles validates the indexed geometry in command.
func ValidateTriangles(command DrawTriangles) error {
	if len(command.Indices)%3 != 0 {
		return ErrTriangleIndexCount
	}
	for _, index := range command.Indices {
		if uint64(index) >= uint64(len(command.Vertices)) {
			return ErrTriangleIndexRange
		}
	}
	return nil
}

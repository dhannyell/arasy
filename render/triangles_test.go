package render_test

import (
	"errors"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestValidateTriangles(t *testing.T) {
	valid := render.DrawTriangles{
		Vertices: []math2d.Vec2{{}, {X: 1}, {Y: 1}},
		Indices:  []uint32{0, 1, 2},
	}
	if err := render.ValidateTriangles(valid); err != nil {
		t.Fatal(err)
	}
	invalidCount := valid
	invalidCount.Indices = []uint32{0, 1}
	if err := render.ValidateTriangles(invalidCount); !errors.Is(err, render.ErrTriangleIndexCount) {
		t.Fatalf("count error = %v", err)
	}
	invalidIndex := valid
	invalidIndex.Indices = []uint32{0, 1, 3}
	if err := render.ValidateTriangles(invalidIndex); !errors.Is(err, render.ErrTriangleIndexRange) {
		t.Fatalf("range error = %v", err)
	}
}

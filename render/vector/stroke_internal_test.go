package vector

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestOuterJoinPointsSelectsOutsideOfBothTurnDirections(t *testing.T) {
	tests := []struct {
		name                string
		prev, center, next  math2d.Vec2
		wantPositiveOutside bool
	}{
		{
			name: "positive turn opens above",
			prev: math2d.Vec2{X: -1, Y: 1}, center: math2d.Vec2{},
			next: math2d.Vec2{X: 1, Y: 1},
		},
		{
			name: "negative turn opens below",
			prev: math2d.Vec2{X: -1, Y: -1}, center: math2d.Vec2{},
			next: math2d.Vec2{X: 1, Y: -1}, wantPositiveOutside: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, b, ok := outerJoinPoints(test.prev, test.center, test.next, 1)
			if !ok {
				t.Fatal("outerJoinPoints rejected a valid turn")
			}
			if test.wantPositiveOutside {
				if a.Y <= 0 || b.Y <= 0 {
					t.Fatalf("outside points = %v,%v, want positive Y", a, b)
				}
			} else if a.Y >= 0 || b.Y >= 0 {
				t.Fatalf("outside points = %v,%v, want negative Y", a, b)
			}
		})
	}
}

func TestOuterJoinPointsSkipsStraightSegments(t *testing.T) {
	_, _, ok := outerJoinPoints(
		math2d.Vec2{X: -1}, math2d.Vec2{}, math2d.Vec2{X: 1}, 1,
	)
	if ok {
		t.Fatal("straight segments unexpectedly produced a join")
	}
}

func TestMiterJoinCoversCenterAndOutsideCorner(t *testing.T) {
	var vertices []math2d.Vec2
	var indices []uint32
	center := math2d.Vec2{}
	ok := appendMiterJoin(
		&vertices, &indices,
		math2d.Vec2{X: -1, Y: 1}, center, math2d.Vec2{X: 1, Y: 1},
		1, 4,
	)
	if !ok {
		t.Fatal("appendMiterJoin rejected a valid turn")
	}
	if len(vertices) != 4 || len(indices) != 6 {
		t.Fatalf("miter geometry has %d vertices and %d indices, want 4 and 6", len(vertices), len(indices))
	}
	if vertices[0] != center {
		t.Fatalf("first miter vertex = %v, want center %v", vertices[0], center)
	}
	used := make(map[uint32]bool)
	for _, index := range indices {
		used[index] = true
	}
	for index := range vertices {
		if !used[uint32(index)] {
			t.Fatalf("miter vertex %d is not referenced", index)
		}
	}
}

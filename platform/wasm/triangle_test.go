package wasm

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestInflateTriangleOffsetsEveryEdgeOutwards(t *testing.T) {
	triangles := []struct {
		name     string
		triangle [3]math2d.Vec2
	}{
		{name: "counterclockwise", triangle: [3]math2d.Vec2{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}}},
		{name: "clockwise", triangle: [3]math2d.Vec2{{X: 0, Y: 0}, {X: 0, Y: 10}, {X: 10, Y: 0}}},
	}
	for _, tc := range triangles {
		t.Run(tc.name, func(t *testing.T) {
			triangle := tc.triangle
			got := inflateTriangle(triangle, 1)
			orientation := math.Copysign(1, cross(triangle[1].Sub(triangle[0]), triangle[2].Sub(triangle[0])))

			for i := range triangle {
				start := triangle[i]
				end := triangle[(i+1)%len(triangle)]
				for _, point := range []math2d.Vec2{got[i], got[(i+1)%len(got)]} {
					distance := orientation * signedDistanceToEdge(start, end, point)
					if math.Abs(distance+1) > 1e-9 {
						t.Fatalf("inflated edge distance = %v, want -1 for point %v on edge %v -> %v", distance, point, start, end)
					}
				}
			}
		})
	}
}

func TestInflateTrianglePreservesDegenerateTriangle(t *testing.T) {
	triangle := [3]math2d.Vec2{{X: 2, Y: 2}, {X: 2, Y: 2}, {X: 4, Y: 4}}
	if got := inflateTriangle(triangle, 0.6); got != triangle {
		t.Fatalf("inflateTriangle(degenerate) = %v, want %v", got, triangle)
	}
}

func TestInflateTriangleNonPositiveAmountIsNoOp(t *testing.T) {
	triangle := [3]math2d.Vec2{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}}
	if got := inflateTriangle(triangle, 0); got != triangle {
		t.Fatalf("inflateTriangle(amount=0) = %v, want %v", got, triangle)
	}
}

func signedDistanceToEdge(start, end, point math2d.Vec2) float64 {
	edge := end.Sub(start)
	return cross(edge, point.Sub(start)) / math.Hypot(edge.X, edge.Y)
}

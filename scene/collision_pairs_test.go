package scene

import (
	"slices"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestSpatialHashGridEntityPairsMatchesReferenceAcrossWorldShapes(t *testing.T) {
	tests := []struct {
		name  string
		build func(*World)
	}{
		{
			name: "spread",
			build: func(w *World) {
				for i := range 12 {
					addGridTestCircle(w, math2d.Vec2{X: float64(i * 8), Y: float64((i % 3) * 8)}, 1)
				}
			},
		},
		{
			name: "dense",
			build: func(w *World) {
				for i := range 8 {
					addGridTestCircle(w, math2d.Vec2{X: float64(i % 4), Y: float64(i / 4)}, 3)
				}
			},
		},
		{
			name: "mixed-large-colliders",
			build: func(w *World) {
				addGridTestCircle(w, math2d.Vec2{X: -3, Y: -3}, 5)
				addGridTestCircle(w, math2d.Vec2{X: 4, Y: 1}, 4)
				addGridTestRect(w, math2d.Vec2{X: 1, Y: 4}, math2d.Vec2{X: 5, Y: 2})
				addGridTestRect(w, math2d.Vec2{X: 12, Y: 4}, math2d.Vec2{X: 6, Y: 3})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := NewWorld()
			test.build(w)
			grid := newSpatialHashGrid(4)
			grid.populate(w)

			got := grid.entityPairs()
			want := referenceEntityPairs(&grid)
			if !slices.Equal(got, want) {
				t.Fatalf("entityPairs() = %v, want %v", got, want)
			}
		})
	}
}

func addGridTestCircle(w *World, position math2d.Vec2, radius float64) {
	id := w.Spawn()
	w.SetTransform(id, Transform{Position: position})
	w.SetCircleCollider(id, CircleCollider{Radius: radius})
}

func addGridTestRect(w *World, position, halfExtents math2d.Vec2) {
	id := w.Spawn()
	w.SetTransform(id, Transform{Position: position})
	w.SetRectCollider(id, RectCollider{HalfExtends: halfExtents})
}

func referenceEntityPairs(g *spatialHashGrid) []entityPair {
	pairSet := make(map[entityPair]struct{})
	cells := g.sortedCells()
	for _, cell := range cells {
		entities := append([]EntityID(nil), g.cells[cell]...)
		slices.Sort(entities)
		for i := range entities {
			for j := i + 1; j < len(entities); j++ {
				if entities[i] != entities[j] {
					pairSet[makeEntityPair(entities[i], entities[j])] = struct{}{}
				}
			}
		}
	}

	pairs := make([]entityPair, 0, len(pairSet))
	for pair := range pairSet {
		pairs = append(pairs, pair)
	}
	slices.SortFunc(pairs, compareEntityPair)
	return pairs
}

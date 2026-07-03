package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func TestWorldDetectCircleOverlapsGridMatchesNarrowPhase(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)
	spawnCollidableCircle(world, math2d.Vec2{X: 25, Y: 0}, 2)
	spawnCollidableCircle(world, math2d.Vec2{X: 26, Y: 0}, 2)

	narrowContacts := world.DetectCircleOverlaps()
	gridContacts := world.DetectCircleOverlapsGrid(16)

	assertContactPairs(t, gridContacts, contactPairs(narrowContacts))
}

// TestWorldDetectOverlapsGridMatchesAllPairsWithRects proves the grid
// broad-phase agrees with an all-pairs oracle on a world mixing circles and
// rects. DetectCircleOverlaps() cannot be used as the oracle here: it only
// builds pairs from circleColliderEntities() and ignores rects entirely, so
// it would silently drop every rect-involving contact. Instead, the oracle is
// the grid itself with a single giant cell (cellSize 10000 for a 60x60
// world), which forces it to fall back to comparing every pair â€” equivalent
// to all-pairs, but through the code path that actually sees rects.
func TestWorldDetectOverlapsGridMatchesAllPairsWithRects(t *testing.T) {
	world := benchmarkWorldSpreadMixed(36, 60, 60, 6, 2)
	clone := world.Clone()

	small := world.DetectCircleOverlapsGrid(8)
	large := clone.DetectCircleOverlapsGrid(10000)

	if len(small) == 0 {
		t.Fatal("expected at least one contact, got 0")
	}

	if len(small) != len(large) {
		t.Fatalf("got %d contacts from small grid, %d from single-cell grid", len(small), len(large))
	}

	for i := range small {
		if small[i].A != large[i].A || small[i].B != large[i].B {
			t.Fatalf("contact %d got pair (%d,%d), want (%d,%d)", i, small[i].A, small[i].B, large[i].A, large[i].B)
		}
		if small[i].Normal != large[i].Normal {
			t.Fatalf("contact %d got normal %+v, want %+v", i, small[i].Normal, large[i].Normal)
		}
		if small[i].Depth != large[i].Depth {
			t.Fatalf("contact %d got depth %v, want %v", i, small[i].Depth, large[i].Depth)
		}
	}
}

func TestWorldDetectCircleOverlapsGridDoesNotDuplicateLargeColliderContacts(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 8, Y: 8}, 10)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 12, Y: 8}, 10)

	contacts := world.DetectCircleOverlapsGrid(4)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
	})
}

func TestWorldDetectCircleOverlapsGridHandlesNegativeCoordinates(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: -3, Y: -3}, 3)
	second := spawnCollidableCircle(world, math2d.Vec2{X: -1, Y: -3}, 3)

	contacts := world.DetectCircleOverlapsGrid(4)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
	})
}

func TestWorldDetectCircleOverlapsGridReturnsContactsInEntityOrder(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)
	third := spawnCollidableCircle(world, math2d.Vec2{X: 6, Y: 0}, 5)

	contacts := world.DetectCircleOverlapsGrid(4)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
		{first, third},
		{second, third},
	})
}

func TestWorldDetectCircleOverlapsGridFallsBackWithInvalidCellSize(t *testing.T) {
	world := scene.NewWorld()

	first := spawnCollidableCircle(world, math2d.Vec2{X: 0, Y: 0}, 5)
	second := spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 0}, 5)

	contacts := world.DetectCircleOverlapsGrid(0)

	assertContactPairs(t, contacts, [][2]scene.EntityID{
		{first, second},
	})
}

func TestWorldDetectCircleOverlapsGridMarksCandidateDebugCells(t *testing.T) {
	world := scene.NewWorld()

	spawnCollidableCircle(world, math2d.Vec2{X: 2, Y: 2}, 0.5)
	spawnCollidableCircle(world, math2d.Vec2{X: 3, Y: 2}, 0.5)
	spawnCollidableCircle(world, math2d.Vec2{X: 18, Y: 18}, 0.5)

	world.DetectCircleOverlapsGrid(8)

	var occupied int
	var candidates int
	for _, cell := range world.DebugGridCells() {
		occupied++
		if cell.Candidate {
			candidates++
		}
	}

	if occupied != 2 {
		t.Fatalf("occupied debug cells = %d, want 2", occupied)
	}

	if candidates != 1 {
		t.Fatalf("candidate debug cells = %d, want 1", candidates)
	}
}

package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// TestCollisionGridWordBoundaries proves the bitset addressing stays correct
// where a row crosses a 64-bit word: cells 63 and 64 live in different words.
func TestCollisionGridWordBoundaries(t *testing.T) {
	grid := scene.NewCollisionGrid(130, 2, math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1})

	grid.SetSolid(63, 0, true)
	grid.SetSolid(64, 1, true)
	grid.SetSolid(129, 1, true)

	if !grid.Solid(63, 0) || !grid.Solid(64, 1) || !grid.Solid(129, 1) {
		t.Fatal("set cells must read back solid")
	}
	// Rows must not share words: cell 64 of row 1 stays out of row 0.
	if grid.Solid(64, 0) || grid.Solid(63, 1) || grid.Solid(129, 0) {
		t.Fatal("cells in other rows must stay empty")
	}

	grid.SetSolid(63, 0, false)
	if grid.Solid(63, 0) {
		t.Fatal("cleared cell must read back empty")
	}
}

func TestCollisionGridIgnoresOutOfRangeCells(t *testing.T) {
	grid := scene.NewCollisionGrid(4, 4, math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1})

	grid.SetSolid(-1, 0, true)
	grid.SetSolid(0, 4, true)

	if grid.Solid(-1, 0) || grid.Solid(0, 4) || grid.Solid(4, 0) {
		t.Fatal("cells outside the grid must be empty")
	}
	for _, word := range grid.Words {
		if word != 0 {
			t.Fatal("out-of-range writes must not touch the bitset")
		}
	}
}

// TestCollisionGridSolidInRect covers the world-to-cell conversion edges: a
// rect edge exactly on a cell boundary must not enter the next cell.
func TestCollisionGridSolidInRect(t *testing.T) {
	grid := scene.NewCollisionGrid(100, 4, math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 2, Y: 2})
	grid.SetSolid(70, 2, true)

	cases := []struct {
		name string
		rect math2d.Rect
		want bool
	}{
		{"over the solid cell", math2d.NewRect(math2d.Vec2{X: 151, Y: 15}, math2d.Vec2{X: 1, Y: 1}), true},
		{"spans many empty words then hits", math2d.NewRect(math2d.Vec2{X: 11, Y: 15}, math2d.Vec2{X: 141, Y: 1}), true},
		{"stops on the cell boundary", math2d.NewRect(math2d.Vec2{X: 11, Y: 15}, math2d.Vec2{X: 139, Y: 1}), false},
		{"wrong row", math2d.NewRect(math2d.Vec2{X: 151, Y: 11}, math2d.Vec2{X: 1, Y: 1}), false},
		{"outside the grid", math2d.NewRect(math2d.Vec2{X: -50, Y: -50}, math2d.Vec2{X: 10, Y: 10}), false},
	}
	for _, c := range cases {
		if got := grid.SolidInRect(c.rect); got != c.want {
			t.Errorf("%s: SolidInRect = %v, want %v", c.name, got, c.want)
		}
	}
}

// wallWorld builds a world with one solid cell at (2, 2) of a 4x4 grid of
// 10-unit cells at the origin: the solid cell spans (20,20)-(30,30).
func wallWorld() *scene.World {
	world := scene.NewWorld()
	grid := scene.NewCollisionGrid(4, 4, math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})
	grid.SetSolid(2, 2, true)
	world.SetCollisionGrid(grid)
	return world
}

// TestWorldStepResolvesGridContact proves a dynamic circle overlapping a
// solid cell is pushed out along the least-penetration axis and its inward
// velocity is reflected, and that the contact reports the hit cell.
func TestWorldStepResolvesGridContact(t *testing.T) {
	world := wallWorld()
	// Circle centered left of the wall face at x=20, overlapping it by 1.
	entity := spawnCollidableCircle(world, math2d.Vec2{X: 17, Y: 25}, 4)
	world.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 5}})

	world.Step(0, scene.StepConfig{Bounds: math2d.NewRect(math2d.Vec2{X: -100, Y: -100}, math2d.Vec2{X: 200, Y: 200})})

	contacts := world.GridContacts()
	if len(contacts) != 1 {
		t.Fatalf("grid contacts = %d, want 1", len(contacts))
	}
	contact := contacts[0]
	if contact.Entity != entity || contact.CellX != 2 || contact.CellY != 2 {
		t.Fatalf("contact = %+v, want entity %d at cell (2, 2)", contact, entity)
	}
	if contact.Normal.X != -1 || contact.Normal.Y != 0 || contact.Depth != 1 {
		t.Fatalf("contact normal/depth = %+v/%v, want (-1, 0)/1", contact.Normal, contact.Depth)
	}
	transform, _ := world.Transform(entity)
	if transform.Position.X != 16 || transform.Position.Y != 25 {
		t.Fatalf("position = %+v, want pushed out to (16, 25)", transform.Position)
	}
	velocity, _ := world.Velocity(entity)
	// Restitution 1 reflects the inward velocity: +5 becomes -5.
	if velocity.Linear.X != -5 {
		t.Fatalf("velocity.X = %v, want -5 after reflection", velocity.Linear.X)
	}
}

// TestWorldStepGridContactStaticEntity proves an immovable entity still
// reports the contact without moving: games read it, physics skips it.
func TestWorldStepGridContactStaticEntity(t *testing.T) {
	world := wallWorld()
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Position: math2d.Vec2{X: 17, Y: 25}, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleCollider(entity, scene.CircleCollider{Radius: 4})

	world.Step(0, scene.StepConfig{Bounds: math2d.NewRect(math2d.Vec2{X: -100, Y: -100}, math2d.Vec2{X: 200, Y: 200})})

	if len(world.GridContacts()) != 1 {
		t.Fatalf("grid contacts = %d, want 1", len(world.GridContacts()))
	}
	transform, _ := world.Transform(entity)
	if transform.Position.X != 17 {
		t.Fatalf("position.X = %v, want unchanged 17", transform.Position.X)
	}
}

// TestCollisionGridCloneAndChecksum proves the grid is simulation state: the
// clone owns its bitset, and SimChecksum sees the grid and its mutations.
func TestCollisionGridCloneAndChecksum(t *testing.T) {
	world := wallWorld()
	base := scene.NewWorld()
	if world.SimChecksum() == base.SimChecksum() {
		t.Fatal("setting a collision grid must change SimChecksum")
	}

	clone := world.Clone()
	if clone.SimChecksum() != world.SimChecksum() {
		t.Fatal("clone must reproduce the grid checksum")
	}

	world.CollisionGrid().SetSolid(0, 0, true)
	if clone.CollisionGrid().Solid(0, 0) {
		t.Fatal("clone shares the bitset with the source (shallow copy)")
	}
	if clone.SimChecksum() == world.SimChecksum() {
		t.Fatal("mutating the source grid must diverge the checksums")
	}
}

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

// TestDebugCommandsDrawCollisionGridCells proves the colliders overlay shows
// the collision grid and merges each horizontal run of solid cells into one
// rect: the merge is what keeps the command count low on long floors.
func TestDebugCommandsDrawCollisionGridCells(t *testing.T) {
	world := scene.NewWorld()
	grid := scene.NewCollisionGrid(4, 4, math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})
	grid.SetSolid(0, 2, true)
	grid.SetSolid(1, 2, true)
	grid.SetSolid(2, 2, true)
	grid.SetSolid(3, 0, true)
	world.SetCollisionGrid(grid)

	var rects []render.DrawRect
	for _, cmd := range world.DebugCommands(scene.DebugColliders) {
		if rect, ok := cmd.(render.DrawRect); ok {
			rects = append(rects, rect)
		}
	}
	if len(rects) != 2 {
		t.Fatalf("grid overlay rects = %d, want 2 (one per solid run)", len(rects))
	}
	run := math2d.NewRect(math2d.Vec2{X: 0, Y: 20}, math2d.Vec2{X: 30, Y: 10})
	if rects[0].Rect != run && rects[1].Rect != run {
		t.Fatalf("no rect covers the merged run %v: %v", run, rects)
	}

	// The overlay is cached; mutating the grid must invalidate it.
	world.CollisionGrid().SetSolid(3, 0, false)
	var after int
	for _, cmd := range world.DebugCommands(scene.DebugColliders) {
		if _, ok := cmd.(render.DrawRect); ok {
			after++
		}
	}
	if after != 1 {
		t.Fatalf("rects after clearing a cell = %d, want 1", after)
	}
}

// TestDebugCommandsDrawGridContacts proves the contacts overlay includes grid
// contacts: without them a body resting on imported terrain shows no contact.
func TestDebugCommandsDrawGridContacts(t *testing.T) {
	world := wallWorld()
	entity := spawnCollidableCircle(world, math2d.Vec2{X: 17, Y: 25}, 4)
	world.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 5}})

	world.Step(0, scene.StepConfig{Bounds: math2d.NewRect(math2d.Vec2{X: -100, Y: -100}, math2d.Vec2{X: 200, Y: 200})})

	var points []render.DrawPoint
	var lines []render.DrawLine
	for _, cmd := range world.DebugCommands(scene.DebugContacts) {
		switch c := cmd.(type) {
		case render.DrawPoint:
			points = append(points, c)
		case render.DrawLine:
			lines = append(lines, c)
		}
	}
	if len(points) != 1 || len(lines) != 1 {
		t.Fatalf("grid contact overlay = %d points, %d lines, want 1 and 1", len(points), len(lines))
	}
	// The wall pushed with normal (-1, 0); the normal line must point -X.
	if lines[0].To.X >= lines[0].From.X {
		t.Fatalf("grid contact normal line must point away from the wall: %+v", lines[0])
	}
}

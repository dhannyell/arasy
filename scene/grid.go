package scene

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

// CollisionGrid is a binary occupancy grid in world coordinates. It is
// simulation state.
type CollisionGrid struct {
	Words    []uint64    // One bit per cell, row-major; each row starts on a word boundary.
	W, H     int         // Grid size in cells.
	Origin   math2d.Vec2 // World position of cell (0,0).
	CellSize math2d.Vec2 // World size of one cell.
}

// NewCollisionGrid allocates an empty grid of w by h cells.
func NewCollisionGrid(w, h int, origin, cellSize math2d.Vec2) CollisionGrid {
	if w <= 0 || h <= 0 {
		return CollisionGrid{Origin: origin, CellSize: cellSize}
	}
	return CollisionGrid{
		Words:    make([]uint64, ((w+63)/64)*h),
		W:        w,
		H:        h,
		Origin:   origin,
		CellSize: cellSize,
	}
}

func (g *CollisionGrid) wordsPerRow() int {
	return (g.W + 63) / 64
}

// Solid reports whether the cell is solid. Cells outside the grid are empty.
func (g *CollisionGrid) Solid(x, y int) bool {
	if x < 0 || y < 0 || x >= g.W || y >= g.H {
		return false
	}
	return g.Words[y*g.wordsPerRow()+x/64]&(1<<uint(x%64)) != 0
}

// SetSolid changes one cell. Cells outside the grid are ignored.
func (g *CollisionGrid) SetSolid(x, y int, solid bool) {
	if x < 0 || y < 0 || x >= g.W || y >= g.H {
		return
	}
	index := y*g.wordsPerRow() + x/64
	bit := uint64(1) << uint(x%64)
	if solid {
		g.Words[index] |= bit
	} else {
		g.Words[index] &^= bit
	}
}

// CellsOf converts a world rect into the inclusive cell range it overlaps,
// clamped to the grid. ok is false when the rect misses the grid entirely.
func (g *CollisionGrid) CellsOf(r math2d.Rect) (minX, minY, maxX, maxY int, ok bool) {
	if g.W <= 0 || g.H <= 0 || g.CellSize.X <= 0 || g.CellSize.Y <= 0 {
		return 0, 0, 0, 0, false
	}
	minX = int(math.Floor((r.Min.X - g.Origin.X) / g.CellSize.X))
	minY = int(math.Floor((r.Min.Y - g.Origin.Y) / g.CellSize.Y))
	// A rect edge exactly on a cell boundary does not enter the next cell.
	maxX = int(math.Ceil((r.Max.X-g.Origin.X)/g.CellSize.X)) - 1
	maxY = int(math.Ceil((r.Max.Y-g.Origin.Y)/g.CellSize.Y)) - 1
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	if maxX < 0 || maxY < 0 || minX >= g.W || minY >= g.H {
		return 0, 0, 0, 0, false
	}
	return max(minX, 0), max(minY, 0), min(maxX, g.W-1), min(maxY, g.H-1), true
}

// SolidInRect reports whether any cell overlapped by the world rect is solid.
func (g *CollisionGrid) SolidInRect(r math2d.Rect) bool {
	minX, minY, maxX, maxY, ok := g.CellsOf(r)
	if !ok {
		return false
	}
	for y := minY; y <= maxY; y++ {
		if g.rowSolidInRange(y, minX, maxX) {
			return true
		}
	}
	return false
}

// GridContact is one contact between an entity and the collision grid. The
// grid is static, so there is no pair: only the entity moves in response.
type GridContact struct {
	Entity EntityID
	CellX  int
	CellY  int
	// Normal points from the grid cell toward the entity.
	Normal math2d.Vec2
	Depth  float64
}

// SetCollisionGrid installs the world's collision grid. The world keeps its
// own copy; mutate it later through CollisionGrid.
func (w *World) SetCollisionGrid(g CollisionGrid) {
	g.Words = append([]uint64(nil), g.Words...)
	w.collisionGrid = &g
}

// CollisionGrid returns the world's collision grid, or nil when none is set.
func (w *World) CollisionGrid() *CollisionGrid {
	return w.collisionGrid
}

// GridContacts returns the grid contacts detected in the last Step.
func (w *World) GridContacts() []GridContact {
	return w.gridContacts
}

// collideWithGrid pushes collider entities out of solid cells and records the
// contacts. It runs before pair collision so the static world settles first.
// Cells resolve in row-major order per entity; the position updates between
// cells, so sliding along a wall stays stable.
func (w *World) collideWithGrid() {
	g := w.collisionGrid
	if g == nil || g.W <= 0 || g.H <= 0 {
		return
	}
	for _, id := range w.circleColliders.ids {
		collider, _ := w.circleColliders.get(id)
		half := math2d.Vec2{X: collider.Radius, Y: collider.Radius}
		w.collideEntityWithGrid(id, half)
	}
	for _, id := range w.rectColliders.ids {
		if _, both := w.circleColliders.get(id); both {
			continue
		}
		collider, _ := w.rectColliders.get(id)
		w.collideEntityWithGrid(id, collider.HalfExtends)
	}
}

// collideEntityWithGrid resolves one entity AABB against every overlapped
// solid cell. Circles use their bounding box in v1.
func (w *World) collideEntityWithGrid(id EntityID, half math2d.Vec2) {
	g := w.collisionGrid
	if g == nil {
		return
	}
	transform, ok := w.transforms.get(id)
	if !ok {
		return
	}
	body := w.bodyOrDefault(id)
	moved := false
	entity := math2d.NewRect(transform.Position.Sub(half), half.Mul(2))
	minX, minY, maxX, maxY, inGrid := g.CellsOf(entity)
	if !inGrid {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !g.Solid(x, y) {
				continue
			}
			entity = math2d.NewRect(transform.Position.Sub(half), half.Mul(2))
			cell := math2d.NewRect(
				g.Origin.Add(math2d.Vec2{X: float64(x) * g.CellSize.X, Y: float64(y) * g.CellSize.Y}),
				g.CellSize,
			)
			normal, depth, hit := aabbCellPenetration(entity, cell)
			if !hit {
				continue
			}
			w.gridContacts = append(w.gridContacts, GridContact{
				Entity: id, CellX: x, CellY: y, Normal: normal, Depth: depth,
			})
			if body.InvMass <= 0 {
				continue
			}
			transform.Position = transform.Position.Add(normal.Mul(depth))
			moved = true
			if velocity, hasVelocity := w.Velocity(id); hasVelocity {
				// Sign differs from resolveContacts: this normal points at the
				// entity, so approaching is negative.
				vNormal := float64(velocity.Linear.X*normal.X) + float64(velocity.Linear.Y*normal.Y)
				if vNormal < 0 {
					velocity.Linear = velocity.Linear.Sub(normal.Mul((1 + body.Restitution) * vNormal))
					w.velocities.set(id, velocity)
				}
			}
		}
	}
	if moved {
		w.transforms.set(id, transform)
	}
}

// aabbCellPenetration reports how a moving entity AABB penetrates one solid
// cell AABB: the push normal (from the cell toward the entity), the depth
// along that normal, and whether they overlap at all. The push axis is the
// axis of least penetration; ties push on X.
func aabbCellPenetration(entity, cell math2d.Rect) (math2d.Vec2, float64, bool) {
	// aabbPenetration points a->b; the cell first makes the normal push outward.
	return aabbPenetration(cell, entity)
}

// debugSolidCellRects returns one world rect per horizontal run of solid
// cells, row by row. Merging runs keeps the debug overlay's command count low
// on maps with long solid floors.
func (g *CollisionGrid) debugSolidCellRects() []math2d.Rect {
	var rects []math2d.Rect
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; {
			if !g.Solid(x, y) {
				x++
				continue
			}
			start := x
			for x < g.W && g.Solid(x, y) {
				x++
			}
			rects = append(rects, math2d.NewRect(
				g.Origin.Add(math2d.Vec2{X: float64(start) * g.CellSize.X, Y: float64(y) * g.CellSize.Y}),
				math2d.Vec2{X: float64(x-start) * g.CellSize.X, Y: g.CellSize.Y},
			))
		}
	}
	return rects
}

// rowSolidInRange reports whether any cell in [minX, maxX] of row y is solid.
func (g *CollisionGrid) rowSolidInRange(y, minX, maxX int) bool {
	rowStart := y * g.wordsPerRow()
	startWord, endWord := minX/64, maxX/64

	for w := startWord; w <= endWord; w++ {
		word := g.Words[rowStart+w]
		if word == 0 {
			continue
		}

		start, end := 0, 63
		if w == startWord {
			start = minX % 64
		}
		if w == endWord {
			end = maxX % 64
		}

		mask := ^uint64(0) << start & (^uint64(0) >> (63 - end))

		if word&mask != 0 {
			return true
		}
	}

	return false
}

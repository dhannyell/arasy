package scene

import (
	"math"
	"slices"

	"github.com/dhannyell/frameforge/math2d"
)

type gridCell struct {
	X int
	Y int
}

type entityPair struct {
	A EntityID
	B EntityID
}

type spatialHashGrid struct {
	cellSize float64
	cells    map[gridCell][]EntityID
}

type CircleContact struct {
	A EntityID
	B EntityID

	// Normal points from A to B
	Normal math2d.Vec2

	// Depth is how much the two circles overlap.
	Depth float64
}

func newSpatialHashGrid(cellSize float64) spatialHashGrid {
	if cellSize <= 0 {
		cellSize = defaultCellSize
	}

	return spatialHashGrid{
		cellSize: cellSize,
		cells:    make(map[gridCell][]EntityID),
	}
}

func (w *World) BounceCirclesInside(bounds math2d.Rect) {
	for id, collider := range w.CircleColliders {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		velocity, ok := w.Velocities[id]
		if !ok {
			continue
		}

		radius := collider.Radius

		if transform.Position.X-radius < bounds.Min.X {
			transform.Position.X = bounds.Min.X + radius
			velocity.Linear.X = -velocity.Linear.X
		}

		if transform.Position.X+radius > bounds.Max.X {
			transform.Position.X = bounds.Max.X - radius
			velocity.Linear.X = -velocity.Linear.X
		}

		if transform.Position.Y-radius < bounds.Min.Y {
			transform.Position.Y = bounds.Min.Y + radius
			velocity.Linear.Y = -velocity.Linear.Y
		}

		if transform.Position.Y+radius > bounds.Max.Y {
			transform.Position.Y = bounds.Max.Y - radius
			velocity.Linear.Y = -velocity.Linear.Y
		}

		w.Transforms[id] = transform
		w.Velocities[id] = velocity
	}
}

func (w *World) DetectCircleOverlaps() []CircleContact {
	entities := w.circleColliderEntities()
	pairs := make([]entityPair, 0, len(entities))

	for i := range entities {
		for j := i + 1; j < len(entities); j++ {
			pairs = append(pairs, makeEntityPair(entities[i], entities[j]))
		}
	}

	return w.detectCircleOverlapsForPairs(pairs)
}

func (w *World) DetectCircleOverlapsGrid(cellSize float64) []CircleContact {
	if cellSize <= 0 {
		w.DebugGridCells = nil
		return w.DetectCircleOverlaps()
	}

	grid := newSpatialHashGrid(cellSize)
	grid.populate(w)
	w.DebugGridCells = grid.debugCells()

	return w.detectCircleOverlapsForPairs(grid.entityPairs())
}

func (w *World) detectCircleOverlapsForPairs(pairs []entityPair) []CircleContact {
	contacts := make([]CircleContact, 0)

	for _, pair := range pairs {
		if contact, ok := w.detectCircleOverlapPair(pair); ok {
			contacts = append(contacts, contact)
		}
	}

	return contacts
}

func (w *World) detectCircleOverlapPair(pair entityPair) (CircleContact, bool) {
	transformA, ok := w.Transforms[pair.A]
	if !ok {
		return CircleContact{}, false
	}

	transformB, ok := w.Transforms[pair.B]
	if !ok {
		return CircleContact{}, false
	}

	colliderA := w.CircleColliders[pair.A]
	colliderB := w.CircleColliders[pair.B]

	delta := transformB.Position.Sub(transformA.Position)
	distanceSq := delta.LenSq()
	radiusSum := colliderA.Radius + colliderB.Radius
	radiusSumSq := radiusSum * radiusSum

	if distanceSq >= radiusSumSq {
		return CircleContact{}, false
	}

	if distanceSq == 0 {
		return CircleContact{
			A:      pair.A,
			B:      pair.B,
			Normal: math2d.Vec2{X: 1, Y: 0},
			Depth:  radiusSum,
		}, true
	}

	distance := math.Sqrt(distanceSq)
	return CircleContact{
		A:      pair.A,
		B:      pair.B,
		Normal: delta.Div(distance),
		Depth:  radiusSum - distance,
	}, true
}

func (w *World) ResolveCircleOverlaps() {
	contacts := w.DetectCircleOverlaps()
	w.resolveCircleContacts(contacts)
}

func (w *World) ResolveCircleOverlapsGrid(cellSize float64) {
	contacts := w.DetectCircleOverlapsGrid(cellSize)
	w.resolveCircleContacts(contacts)
}

func (w *World) resolveCircleContacts(contacts []CircleContact) {
	for _, contact := range contacts {
		transformA, ok := w.Transforms[contact.A]
		if !ok {
			continue
		}

		transformB, ok := w.Transforms[contact.B]
		if !ok {
			continue
		}

		bodyA := w.bodyOrDefault(contact.A)
		bodyB := w.bodyOrDefault(contact.B)
		totalInvMass := bodyA.InvMass + bodyB.InvMass
		if totalInvMass == 0 {
			continue
		}

		correctionA := contact.Depth * (bodyA.InvMass / totalInvMass)
		correctionB := contact.Depth * (bodyB.InvMass / totalInvMass)

		transformA.Position.X -= contact.Normal.X * correctionA
		transformA.Position.Y -= contact.Normal.Y * correctionA
		transformB.Position.X += contact.Normal.X * correctionB
		transformB.Position.Y += contact.Normal.Y * correctionB

		w.Transforms[contact.A] = transformA
		w.Transforms[contact.B] = transformB

		velocityA, ok := w.Velocity(contact.A)
		if !ok {
			continue
		}

		velocityB, ok := w.Velocity(contact.B)
		if !ok {
			continue
		}

		vRel := velocityA.Linear.Sub(velocityB.Linear)
		vRelNormal := (vRel.X * contact.Normal.X) + (vRel.Y * contact.Normal.Y)

		// it means the objects are approaching each other.
		// When the objects are already moving apart, there is no need to update the relative velocity.
		if vRelNormal > 0 {
			restitution := minFloat(bodyA.Restitution, bodyB.Restitution)
			impulseMagnitude := (1 + restitution) * vRelNormal / totalInvMass
			impulse := math2d.Vec2{
				X: impulseMagnitude * contact.Normal.X,
				Y: impulseMagnitude * contact.Normal.Y,
			}

			velocityA.Linear = velocityA.Linear.Sub(impulse.Mul(bodyA.InvMass))
			velocityB.Linear = velocityB.Linear.Add(impulse.Mul(bodyB.InvMass))

			w.Velocities[contact.A] = velocityA
			w.Velocities[contact.B] = velocityB
		}
	}
}

func (g *spatialHashGrid) populate(w *World) {
	entities := w.circleColliderEntities()

	for _, id := range entities {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		collider := w.CircleColliders[id]

		minCellX := cellCoord(transform.Position.X-collider.Radius, g.cellSize)
		maxCellX := cellCoord(transform.Position.X+collider.Radius, g.cellSize)
		minCellY := cellCoord(transform.Position.Y-collider.Radius, g.cellSize)
		maxCellY := cellCoord(transform.Position.Y+collider.Radius, g.cellSize)

		for cellX := minCellX; cellX <= maxCellX; cellX++ {
			for cellY := minCellY; cellY <= maxCellY; cellY++ {
				cell := gridCell{X: cellX, Y: cellY}
				g.cells[cell] = append(g.cells[cell], id)
			}
		}
	}
}

func (g *spatialHashGrid) entityPairs() []entityPair {
	pairSet := make(map[entityPair]struct{})
	cells := g.sortedCells()

	for _, cell := range cells {
		entities := append([]EntityID(nil), g.cells[cell]...)
		slices.Sort(entities)

		for i := range entities {
			for j := i + 1; j < len(entities); j++ {
				pairSet[makeEntityPair(entities[i], entities[j])] = struct{}{}
			}
		}
	}

	pairs := make([]entityPair, 0, len(pairSet))
	for pair := range pairSet {
		pairs = append(pairs, pair)
	}

	slices.SortFunc(pairs, func(a, b entityPair) int {
		if a.A < b.A {
			return -1
		}

		if a.A > b.A {
			return 1
		}

		if a.B < b.B {
			return -1
		}

		if a.B > b.B {
			return 1
		}

		return 0
	})

	return pairs
}

func (g *spatialHashGrid) debugCells() []DebugGridCell {
	cells := g.sortedCells()
	debugCells := make([]DebugGridCell, 0, len(cells))

	for _, cell := range cells {
		debugCells = append(debugCells, DebugGridCell{
			X:         cell.X,
			Y:         cell.Y,
			Size:      g.cellSize,
			Candidate: len(g.cells[cell]) > 1,
		})
	}

	return debugCells
}

func (g *spatialHashGrid) sortedCells() []gridCell {
	cells := make([]gridCell, 0, len(g.cells))
	for cell, entities := range g.cells {
		if len(entities) > 0 {
			cells = append(cells, cell)
		}
	}

	slices.SortFunc(cells, func(a, b gridCell) int {
		if a.X != b.X {
			return a.X - b.X
		}

		return a.Y - b.Y
	})

	return cells
}

func makeEntityPair(a, b EntityID) entityPair {
	if a > b {
		a, b = b, a
	}

	return entityPair{A: a, B: b}
}

func cellCoord(value, cellSize float64) int {
	return int(math.Floor(value / cellSize))
}

func (w *World) circleColliderEntities() []EntityID {
	entities := make([]EntityID, 0, len(w.CircleColliders))

	for id := range w.CircleColliders {
		entities = append(entities, id)
	}

	slices.Sort(entities)

	return entities
}

func (w *World) bodyOrDefault(id EntityID) Body {
	body, ok := w.Bodies[id]
	if !ok {
		return Body{
			InvMass:     1,
			Restitution: 1,
		}
	}

	if body.InvMass < 0 {
		body.InvMass = 0
	}

	if body.Restitution < 0 {
		body.Restitution = 0
	}

	if body.Restitution > 1 {
		body.Restitution = 1
	}

	return body
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}

	return b
}

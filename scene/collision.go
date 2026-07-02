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

type Contact struct {
	A EntityID
	B EntityID

	// Normal points from A to B
	Normal math2d.Vec2

	// Depth is how much the two circles overlap.
	Depth float64
}

type collisionStats struct {
	contacts        []Contact
	broadPhasePairs int
	gridCells       int
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

func (w *World) BounceEntitiesInside(bounds math2d.Rect) {
	for id, velocity := range w.velocities {
		if velocity.Linear.X == 0 && velocity.Linear.Y == 0 {
			continue
		}

		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		// Determine the size (radius or extent) using fast switches
		var extentX, extentY float64

		if rect, isRect := w.rectColliders[id]; isRect {
			extentX = rect.HalfExtends.X
			extentY = rect.HalfExtends.Y
		} else if circle, isCircle := w.circleColliders[id]; isCircle {
			extentX = circle.Radius
			extentY = circle.Radius
		} else {
			continue
		}

		minXBound := bounds.Min.X + extentX
		if transform.Position.X < minXBound {
			transform.Position.X = minXBound
			if velocity.Linear.X < 0 {
				velocity.Linear.X = -velocity.Linear.X
			}
		} else {
			maxXBound := bounds.Max.X - extentX
			if transform.Position.X > maxXBound {
				transform.Position.X = maxXBound
				if velocity.Linear.X > 0 {
					velocity.Linear.X = -velocity.Linear.X
				}
			}
		}

		minYBound := bounds.Min.Y + extentY
		if transform.Position.Y < minYBound {
			transform.Position.Y = minYBound
			if velocity.Linear.Y < 0 {
				velocity.Linear.Y = -velocity.Linear.Y
			}
		} else {
			maxYBound := bounds.Max.Y - extentY
			if transform.Position.Y > maxYBound {
				transform.Position.Y = maxYBound
				if velocity.Linear.Y > 0 {
					velocity.Linear.Y = -velocity.Linear.Y
				}
			}
		}

		w.transforms[id] = transform
		w.velocities[id] = velocity
	}
}

func (w *World) DetectCircleOverlaps() []Contact {
	entities := w.circleColliderEntities()
	pairs := make([]entityPair, 0, len(entities))

	for i := range entities {
		for j := i + 1; j < len(entities); j++ {
			pairs = append(pairs, makeEntityPair(entities[i], entities[j]))
		}
	}

	return w.detectOverlapsForPairs(pairs)
}

func (w *World) DetectCircleOverlapsGrid(cellSize float64) []Contact {
	if cellSize <= 0 {
		w.debugGridCells = nil
		return w.DetectCircleOverlaps()
	}

	grid := newSpatialHashGrid(cellSize)
	grid.populate(w)
	w.debugGridCells = grid.debugCells()

	return w.detectOverlapsForPairs(grid.entityPairs())
}

func (w *World) detectCircleOverlapsGridStats(cellSize float64) collisionStats {
	if cellSize <= 0 {
		entities := w.circleColliderEntities()
		pairs := make([]entityPair, 0, len(entities))

		for i := range entities {
			for j := i + 1; j < len(entities); j++ {
				pairs = append(pairs, makeEntityPair(entities[i], entities[j]))
			}
		}

		w.debugGridCells = nil

		return collisionStats{
			contacts:        w.detectOverlapsForPairs(pairs),
			broadPhasePairs: len(pairs),
			gridCells:       0,
		}
	}

	grid := newSpatialHashGrid(cellSize)
	grid.populate(w)

	w.debugGridCells = grid.debugCells()
	pairs := grid.entityPairs()

	return collisionStats{
		contacts:        w.detectOverlapsForPairs(pairs),
		broadPhasePairs: len(pairs),
		gridCells:       len(w.debugGridCells),
	}
}

func (w *World) detectOverlapsForPairs(pairs []entityPair) []Contact {
	contacts := make([]Contact, 0)

	for _, pair := range pairs {
		if contact, ok := w.detectOverlapPair(pair); ok {
			contacts = append(contacts, contact)
		}
	}

	return contacts
}

func (w *World) detectOverlapPair(pair entityPair) (Contact, bool) {
	circleA, aIsCircle := w.circleColliders[pair.A]
	circleB, bIsCircle := w.circleColliders[pair.B]
	rectA, aIsRect := w.rectColliders[pair.A]
	rectB, bIsRect := w.rectColliders[pair.B]

	// circle collider override rect collider in same entity
	aIsRect = aIsRect && !aIsCircle
	bIsRect = bIsRect && !bIsCircle

	switch {
	case aIsCircle && bIsCircle:
		return w.circleVsCircle(pair, circleA, circleB)
	case aIsRect && bIsRect:
		return w.rectVsRect(pair, rectA, rectB)
	case aIsCircle && bIsRect:
		return w.circleVsRect(pair.A, pair.B, circleA, rectB, false)
	case aIsRect && bIsCircle:
		return w.circleVsRect(pair.B, pair.A, circleB, rectA, true)
	}

	return Contact{}, false
}

func (w *World) circleVsCircle(pair entityPair, colliderA, colliderB CircleCollider) (Contact, bool) {
	transformA, ok := w.transforms[pair.A]
	if !ok {
		return Contact{}, false
	}

	transformB, ok := w.transforms[pair.B]
	if !ok {
		return Contact{}, false
	}

	delta := transformB.Position.Sub(transformA.Position)
	distanceSq := delta.LenSq()
	radiusSum := colliderA.Radius + colliderB.Radius
	radiusSumSq := radiusSum * radiusSum

	if distanceSq >= radiusSumSq {
		return Contact{}, false
	}

	if distanceSq == 0 {
		return Contact{
			A:      pair.A,
			B:      pair.B,
			Normal: math2d.Vec2{X: 1, Y: 0},
			Depth:  radiusSum,
		}, true
	}

	distance := math.Sqrt(distanceSq)
	return Contact{
		A:      pair.A,
		B:      pair.B,
		Normal: delta.Div(distance),
		Depth:  radiusSum - distance,
	}, true
}

func (w *World) circleVsRect(
	circleID,
	rectID EntityID,
	circleCollider CircleCollider,
	rectCollider RectCollider,
	invertNormal bool,
) (Contact, bool) {
	circleTransform, ok := w.transforms[circleID]
	if !ok {
		return Contact{}, false
	}

	rectTransform, ok := w.transforms[rectID]
	if !ok {
		return Contact{}, false
	}

	minX := rectTransform.Position.X - rectCollider.HalfExtends.X
	maxX := rectTransform.Position.X + rectCollider.HalfExtends.X
	minY := rectTransform.Position.Y - rectCollider.HalfExtends.Y
	maxY := rectTransform.Position.Y + rectCollider.HalfExtends.Y

	// find closest point from circle and rect
	closest := math2d.Vec2{
		X: math2d.Clamp(circleTransform.Position.X, minX, maxX),
		Y: math2d.Clamp(circleTransform.Position.Y, minY, maxY),
	}

	delta := closest.Sub(circleTransform.Position)
	distSq := delta.LenSq()

	if distSq > circleCollider.Radius*circleCollider.Radius {
		return Contact{}, false
	}

	// circle center inside rect
	if distSq == 0 {
		left := circleTransform.Position.X - (rectTransform.Position.X - rectCollider.HalfExtends.X)
		right := (rectTransform.Position.X + rectCollider.HalfExtends.X) - circleTransform.Position.X
		top := circleTransform.Position.Y - (rectTransform.Position.Y - rectCollider.HalfExtends.Y)
		bottom := (rectTransform.Position.Y + rectCollider.HalfExtends.Y) - circleTransform.Position.Y

		minDist := left
		normal := math2d.Vec2{X: -1}

		if right < minDist {
			minDist = right
			normal = math2d.Vec2{X: 1}
		}
		if top < minDist {
			minDist = top
			normal = math2d.Vec2{Y: -1}
		}
		if bottom < minDist {
			normal = math2d.Vec2{Y: 1}
		}

		if invertNormal {
			normal = normal.Mul(-1)
		}

		return Contact{
			A:      circleID,
			B:      rectID,
			Normal: normal,
			Depth:  circleCollider.Radius + minDist,
		}, true
	}

	normal := delta.Normalize()

	if invertNormal {
		normal = normal.Mul(-1)
	}

	// circle center out rect but border touch rect
	return Contact{
		A:      circleID,
		B:      rectID,
		Normal: normal,
		Depth:  circleCollider.Radius - math.Sqrt(distSq),
	}, true
}

func (w *World) rectVsRect(pair entityPair, colliderA, colliderB RectCollider) (Contact, bool) {
	transformA, ok := w.transforms[pair.A]
	if !ok {
		return Contact{}, false
	}

	transformB, ok := w.transforms[pair.B]
	if !ok {
		return Contact{}, false
	}

	centerAX := transformA.Position.X + (transformA.Scale.X * 0.5)
	centerAY := transformA.Position.Y + (transformA.Scale.Y * 0.5)

	minAX := centerAX - colliderA.HalfExtends.X
	maxAX := centerAX + colliderA.HalfExtends.X

	if minAX > (transformB.Position.X+colliderB.HalfExtends.X) ||
		maxAX < (transformB.Position.X+colliderB.HalfExtends.X) {
		return Contact{}, false
	}

	minAY := centerAY - colliderA.HalfExtends.Y
	maxAY := centerAY + colliderA.HalfExtends.Y

	if minAY > (transformB.Position.Y+colliderB.HalfExtends.Y) ||
		maxAY < (transformB.Position.Y-colliderB.HalfExtends.Y) {
		return Contact{}, false
	}

	// Here collision is 100% confirmed, only need to find normal and depth
	minBX := transformB.Position.X - colliderB.HalfExtends.X
	maxBX := transformB.Position.X + colliderB.HalfExtends.X
	minBY := transformB.Position.Y - colliderB.HalfExtends.Y
	maxBY := transformB.Position.Y + colliderB.HalfExtends.Y

	overlapLeft := maxAX - minBX
	overlapRight := maxBX - minAX
	overlapBottom := maxAY - minBY
	overlapTop := maxBY - minAY

	if overlapLeft < overlapRight {
		if overlapLeft < overlapBottom && overlapLeft < overlapTop {
			return Contact{
				A:      pair.A,
				B:      pair.B,
				Normal: math2d.Vec2{X: -1.0, Y: 0.0},
				Depth:  overlapLeft,
			}, true
		}
	} else {
		if overlapRight < overlapBottom && overlapRight < overlapTop {
			return Contact{
				A:      pair.A,
				B:      pair.B,
				Normal: math2d.Vec2{X: 1.0, Y: 0.0},
				Depth:  overlapRight,
			}, true
		}
	}

	if overlapBottom < overlapTop {
		return Contact{
			A:      pair.A,
			B:      pair.B,
			Normal: math2d.Vec2{X: 0.0, Y: -1.0},
			Depth:  overlapLeft,
		}, true
	}

	return Contact{
		A:      pair.A,
		B:      pair.B,
		Normal: math2d.Vec2{X: 0.0, Y: 1.0},
		Depth:  overlapTop,
	}, true
}

func (w *World) ResolveCircleOverlaps() {
	contacts := w.DetectCircleOverlaps()
	w.resolveContacts(contacts)
}

func (w *World) ResolveCircleOverlapsGrid(cellSize float64) {
	contacts := w.DetectCircleOverlapsGrid(cellSize)
	w.resolveContacts(contacts)
}

func (w *World) resolveContacts(contacts []Contact) {
	w.contacts = contacts

	for _, contact := range contacts {
		transformA, ok := w.transforms[contact.A]
		if !ok {
			continue
		}

		transformB, ok := w.transforms[contact.B]
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

		w.transforms[contact.A] = transformA
		w.transforms[contact.B] = transformB

		velocityA, hasVelocityA := w.Velocity(contact.A)
		velocityB, hasVelocityB := w.Velocity(contact.B)

		vRel := velocityA.Linear.Sub(velocityB.Linear)
		vRelNormal := (vRel.X * contact.Normal.X) + (vRel.Y * contact.Normal.Y)

		// it means the objects are approaching each other.
		// When the objects are already moving apart, there is no need to update the relative velocity.
		if vRelNormal > 0 {
			restitution := min(bodyA.Restitution, bodyB.Restitution)
			impulseMagnitude := (1 + restitution) * vRelNormal / totalInvMass
			impulse := math2d.Vec2{
				X: impulseMagnitude * contact.Normal.X,
				Y: impulseMagnitude * contact.Normal.Y,
			}

			velocityA.Linear = velocityA.Linear.Sub(impulse.Mul(bodyA.InvMass))
			velocityB.Linear = velocityB.Linear.Add(impulse.Mul(bodyB.InvMass))

			if hasVelocityA {
				w.velocities[contact.A] = velocityA
			}
			if hasVelocityB {
				w.velocities[contact.B] = velocityB
			}
		}
	}
}

func (g *spatialHashGrid) populate(w *World) {
	circleEntities := w.circleColliderEntities()

	for _, id := range circleEntities {
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		collider := w.circleColliders[id]

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

	rectEntities := w.rectColliderEntities()
	for _, id := range rectEntities {
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		collider := w.rectColliders[id]

		minCellX := cellCoord(transform.Position.X-collider.HalfExtends.X, g.cellSize)
		maxCellX := cellCoord(transform.Position.X+collider.HalfExtends.X, g.cellSize)
		minCellY := cellCoord(transform.Position.Y-collider.HalfExtends.Y, g.cellSize)
		maxCellY := cellCoord(transform.Position.Y+collider.HalfExtends.Y, g.cellSize)

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
	entities := make([]EntityID, 0, len(w.circleColliders))

	for id := range w.circleColliders {
		entities = append(entities, id)
	}

	slices.Sort(entities)

	return entities
}

func (w *World) rectColliderEntities() []EntityID {
	entities := make([]EntityID, 0, len(w.rectColliders))

	for id := range w.rectColliders {
		entities = append(entities, id)
	}

	slices.Sort(entities)

	return entities
}

func (w *World) bodyOrDefault(id EntityID) Body {
	body, ok := w.bodies[id]
	if !ok {
		return Body{
			InvMass:     0,
			Restitution: 0,
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

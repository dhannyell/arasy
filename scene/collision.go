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

	// Depth is how much the two shapes overlap.
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

func (w *World) bounceEntitiesInside(bounds math2d.Rect, cfg BounceConfig) {
	for id, velocity := range w.velocities {
		if velocity.Linear.X == 0 && velocity.Linear.Y == 0 {
			continue
		}

		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		// Determine the size (radius or extent).
		// precedence: a circle collider on the same entity wins over a rect.
		var extentX, extentY float64

		if circle, isCircle := w.circleColliders[id]; isCircle {
			extentX = circle.Radius
			extentY = circle.Radius
		} else if rect, isRect := w.rectColliders[id]; isRect {
			extentX = rect.HalfExtends.X
			extentY = rect.HalfExtends.Y
		} else {
			continue
		}

		factor := wallRestitutionFactor(w, id, cfg)

		if cfg.OpenSides&SideLeft == 0 {
			minXBound := bounds.Min.X + extentX
			if transform.Position.X < minXBound {
				transform.Position.X = minXBound
				if velocity.Linear.X < 0 {
					velocity.Linear.X = -velocity.Linear.X * factor
				}
			}
		}
		if cfg.OpenSides&SideRight == 0 {
			maxXBound := bounds.Max.X - extentX
			if transform.Position.X > maxXBound {
				transform.Position.X = maxXBound
				if velocity.Linear.X > 0 {
					velocity.Linear.X = -velocity.Linear.X * factor
				}
			}
		}

		if cfg.OpenSides&SideTop == 0 {
			minYBound := bounds.Min.Y + extentY
			if transform.Position.Y < minYBound {
				transform.Position.Y = minYBound
				if velocity.Linear.Y < 0 {
					velocity.Linear.Y = -velocity.Linear.Y * factor
				}
			}
		}
		if cfg.OpenSides&SideBottom == 0 {
			maxYBound := bounds.Max.Y - extentY
			if transform.Position.Y > maxYBound {
				transform.Position.Y = maxYBound
				if velocity.Linear.Y > 0 {
					velocity.Linear.Y = -velocity.Linear.Y * factor
				}
			}
		}

		w.transforms[id] = transform
		w.velocities[id] = velocity
	}
}

func wallRestitutionFactor(w *World, id EntityID, cfg BounceConfig) float64 {
	if !cfg.UseBodyRestitution {
		return 1
	}

	body, ok := w.bodies[id]
	if !ok {
		return 1
	}

	if body.Restitution < 0 {
		return 0
	}
	if body.Restitution > 1 {
		return 1
	}
	return body.Restitution
}

func (w *World) detectCircleOverlaps() []Contact {
	entities := w.circleColliderEntities()
	pairs := make([]entityPair, 0, len(entities))

	for i := range entities {
		for j := i + 1; j < len(entities); j++ {
			pairs = append(pairs, makeEntityPair(entities[i], entities[j]))
		}
	}

	return w.detectOverlapsForPairs(pairs)
}

func (w *World) detectCircleOverlapsGrid(cellSize float64) []Contact {
	if cellSize <= 0 {
		w.debugGridCells = nil
		return w.detectCircleOverlaps()
	}

	grid := newSpatialHashGrid(cellSize)
	grid.populate(w)
	w.debugGridCells = grid.debugCells()

	return w.detectOverlapsForPairs(grid.entityPairs())
}

func (w *World) detectCircleOverlapsGridStats(cellSize float64) collisionStats {
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
		// circle is pair.A -> circleIsB=false
		return w.circleVsRect(pair.A, pair.B, circleA, rectB, false)
	case aIsRect && bIsCircle:
		// circle is pair.B -> circleIsB=true (normal flipped to stay A->B)
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

// circleVsRect resolves a circle against an axis-aligned rect. The math always
// runs "circle vs rect" and produces a normal pointing circle->rect; the caller
// tells us whether the circle is the pair's B entity so we can keep the Contact
// convention (A<B by pair order, Normal pointing A->B).
func (w *World) circleVsRect(
	circleID,
	rectID EntityID,
	circleCollider CircleCollider,
	rectCollider RectCollider,
	circleIsB bool,
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

	// find closest point on the rect to the circle center
	closest := math2d.Vec2{
		X: math2d.Clamp(circleTransform.Position.X, minX, maxX),
		Y: math2d.Clamp(circleTransform.Position.Y, minY, maxY),
	}

	delta := closest.Sub(circleTransform.Position) // points circle -> rect
	distSq := delta.LenSq()

	var normal math2d.Vec2 // circle -> rect
	var depth float64

	if distSq == 0 {
		// circle center inside rect: push out through the nearest face. The
		// face normals below point circle->rect (toward the rect center).
		left := circleTransform.Position.X - minX
		right := maxX - circleTransform.Position.X
		top := circleTransform.Position.Y - minY
		bottom := maxY - circleTransform.Position.Y

		minDist := left
		normal = math2d.Vec2{X: 1}

		if right < minDist {
			minDist = right
			normal = math2d.Vec2{X: -1}
		}
		if top < minDist {
			minDist = top
			normal = math2d.Vec2{Y: 1}
		}
		if bottom < minDist {
			minDist = bottom
			normal = math2d.Vec2{Y: -1}
		}

		depth = circleCollider.Radius + minDist
	} else {
		// touching-only (distSq == r²) does not count, matching the >= used by
		// circle×circle and the >0 overlap used by rect×rect.
		if distSq >= circleCollider.Radius*circleCollider.Radius {
			return Contact{}, false
		}

		// circle center outside the rect but within radius of the border
		normal = delta.Div(math.Sqrt(distSq))
		depth = circleCollider.Radius - math.Sqrt(distSq)
	}

	a, b := circleID, rectID
	if circleIsB {
		// keep A<B ordering and flip the normal so it still points A->B
		a, b = rectID, circleID
		normal = normal.Mul(-1)
	}

	return Contact{
		A:      a,
		B:      b,
		Normal: normal,
		Depth:  depth,
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

	minAX := transformA.Position.X - colliderA.HalfExtends.X
	maxAX := transformA.Position.X + colliderA.HalfExtends.X
	minAY := transformA.Position.Y - colliderA.HalfExtends.Y
	maxAY := transformA.Position.Y + colliderA.HalfExtends.Y

	minBX := transformB.Position.X - colliderB.HalfExtends.X
	maxBX := transformB.Position.X + colliderB.HalfExtends.X
	minBY := transformB.Position.Y - colliderB.HalfExtends.Y
	maxBY := transformB.Position.Y + colliderB.HalfExtends.Y

	// Bordas apenas encostando (overlap zero) não geram contato, consistente
	// com o >= do círculo.
	overlapX := min(maxAX, maxBX) - max(minAX, minBX)
	if overlapX <= 0 {
		return Contact{}, false
	}

	overlapY := min(maxAY, maxBY) - max(minAY, minBY)
	if overlapY <= 0 {
		return Contact{}, false
	}

	// Normal no eixo de menor penetração; sinal pela diferença de centros
	// (A->B). Desempate X==Y -> eixo X; centros coincidentes -> +X/+Y.
	if overlapX <= overlapY {
		normalX := 1.0
		if transformB.Position.X < transformA.Position.X {
			normalX = -1.0
		}

		return Contact{
			A:      pair.A,
			B:      pair.B,
			Normal: math2d.Vec2{X: normalX},
			Depth:  overlapX,
		}, true
	}

	normalY := 1.0
	if transformB.Position.Y < transformA.Position.Y {
		normalY = -1.0
	}

	return Contact{
		A:      pair.A,
		B:      pair.B,
		Normal: math2d.Vec2{Y: normalY},
		Depth:  overlapY,
	}, true
}

func (w *World) resolveCircleOverlaps() {
	contacts := w.detectCircleOverlaps()
	w.resolveContacts(contacts)
}

func (w *World) resolveCircleOverlapsGrid(cellSize float64) {
	contacts := w.detectCircleOverlapsGrid(cellSize)
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
		vRelNormal := float64(vRel.X*contact.Normal.X) + float64(vRel.Y*contact.Normal.Y)

		// Positive relative velocity means the shapes are approaching.
		if vRelNormal > 0 {
			restitution := min(bodyA.Restitution, bodyB.Restitution)
			impulseMagnitude := (1 + restitution) * vRelNormal / totalInvMass
			impulse := math2d.Vec2{
				X: impulseMagnitude * contact.Normal.X,
				Y: impulseMagnitude * contact.Normal.Y,
			}

			velocityA.Linear = math2d.Vec2{
				X: velocityA.Linear.X - float64(impulse.X*bodyA.InvMass),
				Y: velocityA.Linear.Y - float64(impulse.Y*bodyA.InvMass),
			}
			velocityB.Linear = math2d.Vec2{
				X: velocityB.Linear.X + float64(impulse.X*bodyB.InvMass),
				Y: velocityB.Linear.Y + float64(impulse.Y*bodyB.InvMass),
			}

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
		// D1 precedence: a circle collider on the same entity wins, so the rect
		// is ignored. Skipping it here also avoids inserting the entity twice
		// (which would create a spurious self-pair in entityPairs).
		if _, isCircle := w.circleColliders[id]; isCircle {
			continue
		}

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
				if entities[i] == entities[j] {
					// never pair an entity with itself (e.g. if it was ever
					// inserted under the same cell twice)
					continue
				}
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

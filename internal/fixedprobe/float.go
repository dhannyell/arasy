package fixedprobe

import (
	"cmp"
	"math"
	"slices"
)

// floatWorld mirrors the scene inner loops over float64 slices. Both mirrors
// share this flat layout, so a fixed-vs-float ratio isolates the scalar type.
type floatWorld struct {
	n          int
	posX, posY []float64
	velX, velY []float64
	isRect     []bool
	extX, extY []float64

	invMass     []float64
	restitution []float64

	ballistic    bool
	z, vz, omega []float64

	grid     map[cellKey][]int32
	cellSize float64
	cells    []cellKey
	pairs    []pairKey
	contacts []floatContact
}

type cellKey struct{ x, y int }

type pairKey struct{ a, b int32 }

type floatContact struct {
	a, b   int32
	nx, ny float64
	depth  float64
}

func newFloatWorld(f *fixture) *floatWorld {
	w := &floatWorld{
		n:      f.n,
		posX:   make([]float64, f.n),
		posY:   make([]float64, f.n),
		velX:   make([]float64, f.n),
		velY:   make([]float64, f.n),
		isRect:      slices.Clone(f.isRect),
		extX:        make([]float64, f.n),
		extY:        make([]float64, f.n),
		invMass:     make([]float64, f.n),
		restitution: make([]float64, f.n),
		grid:        make(map[cellKey][]int32),
	}
	for i := range f.n {
		w.posX[i] = qf(f.posX[i])
		w.posY[i] = qf(f.posY[i])
		w.velX[i] = qf(f.velX[i])
		w.velY[i] = qf(f.velY[i])
		w.extX[i] = qf(f.extX[i])
		w.extY[i] = qf(f.extY[i])
		w.invMass[i] = qf(f.invMass[i])
		w.restitution[i] = qf(f.restitution[i])
	}
	if f.z != nil {
		w.ballistic = true
		w.z = make([]float64, f.n)
		w.vz = make([]float64, f.n)
		w.omega = make([]float64, f.n)
		for i := range f.n {
			w.z[i] = qf(f.z[i])
			w.vz[i] = qf(f.vz[i])
			w.omega[i] = qf(f.omega[i])
		}
	}
	return w
}

// step mirrors the Step order for the mirrored subset: ballistics,
// integration, wall bounce, detection, resolution.
func (w *floatWorld) step(dt, cellSize float64) int {
	if w.ballistic {
		w.advanceBallistics(dt)
	}
	w.integrate(dt)
	w.bounce()
	contacts := w.detect(cellSize)
	w.resolve(contacts)
	return len(contacts)
}

// advanceBallistics mirrors the scene integrator, including the explicit
// float64 conversions that shield products from FMA fusion.
func (w *floatWorld) advanceBallistics(dt float64) {
	gravity := qf(qGravity)
	dragK := qf(qDragK)
	magnusK := qf(qMagnusK)
	bounceDamping := qf(qBounceDamping)
	bounceFriction := qf(qBounceFriction)
	spinDecay := qf(qSpinDecay)
	rollFriction := qf(qRollFriction)
	restVZ := qf(qRestVZ)

	for i := range w.n {
		vx, vy, vz := w.velX[i], w.velY[i], w.vz[i]
		omega := w.omega[i]

		speed := math.Sqrt(float64(vx*vx) + float64(vy*vy) + float64(vz*vz))

		dragStep := float64(float64(dragK*speed) * dt)
		if dragStep > 0 {
			dragScale := 1.0 / (1.0 + dragStep)
			vx *= dragScale
			vy *= dragScale
			vz *= dragScale
		}

		if magnusRate := magnusK * omega; magnusRate != 0 {
			halfStep := float64(float64(magnusRate*dt) * 0.5)
			halfStepSq := float64(halfStep * halfStep)
			denom := 1.0 + halfStepSq
			diagonal := 1.0 - halfStepSq
			offDiagonal := float64(2.0 * halfStep)
			rotatedX := (float64(diagonal*vx) - float64(offDiagonal*vy)) / denom
			rotatedY := (float64(offDiagonal*vx) + float64(diagonal*vy)) / denom
			vx, vy = rotatedX, rotatedY
		}

		w.velX[i] = vx
		w.velY[i] = vy
		vz -= float64(gravity * dt)

		z := w.z[i] + float64(vz*dt)

		if z <= 0 {
			z = 0
			if vz < 0 {
				bounced := -vz * bounceDamping
				if bounced < restVZ {
					vz = 0
				} else {
					vz = bounced
					w.velX[i] *= bounceFriction
					w.velY[i] *= bounceFriction
					omega *= bounceFriction
				}
			}
		}

		if z == 0 && vz == 0 {
			rf := 1.0 / (1.0 + float64(rollFriction*dt))
			w.velX[i] *= rf
			w.velY[i] *= rf
		}

		w.z[i] = z
		w.vz[i] = vz
		w.omega[i] = omega / (1.0 + float64(spinDecay*dt))
	}
}

func (w *floatWorld) integrate(dt float64) {
	for i := range w.n {
		w.posX[i] += float64(w.velX[i] * dt)
		w.posY[i] += float64(w.velY[i] * dt)
	}
}

// bounce mirrors the wall reflection with all four sides closed and the
// benchmark's unit restitution factor.
func (w *floatWorld) bounce() {
	const factor = 1.0
	for i := range w.n {
		if w.velX[i] == 0 && w.velY[i] == 0 {
			continue
		}
		extX, extY := w.extX[i], w.extY[i]

		minXBound := 0 + extX
		if w.posX[i] < minXBound {
			w.posX[i] = minXBound
			if w.velX[i] < 0 {
				w.velX[i] = -w.velX[i] * factor
			}
		}
		maxXBound := worldSide - extX
		if w.posX[i] > maxXBound {
			w.posX[i] = maxXBound
			if w.velX[i] > 0 {
				w.velX[i] = -w.velX[i] * factor
			}
		}

		minYBound := 0 + extY
		if w.posY[i] < minYBound {
			w.posY[i] = minYBound
			if w.velY[i] < 0 {
				w.velY[i] = -w.velY[i] * factor
			}
		}
		maxYBound := worldSide - extY
		if w.posY[i] > maxYBound {
			w.posY[i] = maxYBound
			if w.velY[i] > 0 {
				w.velY[i] = -w.velY[i] * factor
			}
		}
	}
}

// detect mirrors the grid broad phase: populate, sort cells, pair, dedupe,
// then the narrow phase per pair.
func (w *floatWorld) detect(cellSize float64) []floatContact {
	if w.cellSize != cellSize {
		w.cellSize = cellSize
		w.grid = make(map[cellKey][]int32)
	} else {
		for cell, entities := range w.grid {
			w.grid[cell] = entities[:0]
		}
	}

	for i := range w.n {
		if w.isRect[i] {
			continue
		}
		w.insert(int32(i), cellSize)
	}
	for i := range w.n {
		if !w.isRect[i] {
			continue
		}
		w.insert(int32(i), cellSize)
	}

	cells := w.cells[:0]
	for cell, entities := range w.grid {
		if len(entities) > 0 {
			cells = append(cells, cell)
		}
	}
	slices.SortFunc(cells, func(a, b cellKey) int {
		if c := cmp.Compare(a.x, b.x); c != 0 {
			return c
		}
		return cmp.Compare(a.y, b.y)
	})
	w.cells = cells

	pairs := w.pairs[:0]
	for _, cell := range cells {
		entities := w.grid[cell]
		for i := range entities {
			for j := i + 1; j < len(entities); j++ {
				if entities[i] != entities[j] {
					a, b := entities[i], entities[j]
					if a > b {
						a, b = b, a
					}
					pairs = append(pairs, pairKey{a: a, b: b})
				}
			}
		}
	}
	slices.SortFunc(pairs, func(a, b pairKey) int {
		if c := cmp.Compare(a.a, b.a); c != 0 {
			return c
		}
		return cmp.Compare(a.b, b.b)
	})
	pairs = slices.Compact(pairs)
	w.pairs = pairs

	contacts := w.contacts[:0]
	for _, pair := range pairs {
		if contact, ok := w.narrow(pair.a, pair.b); ok {
			contacts = append(contacts, contact)
		}
	}
	w.contacts = contacts
	return contacts
}

func (w *floatWorld) insert(id int32, cellSize float64) {
	minCellX := cellCoordF(w.posX[id]-w.extX[id], cellSize)
	maxCellX := cellCoordF(w.posX[id]+w.extX[id], cellSize)
	minCellY := cellCoordF(w.posY[id]-w.extY[id], cellSize)
	maxCellY := cellCoordF(w.posY[id]+w.extY[id], cellSize)

	for cellX := minCellX; cellX <= maxCellX; cellX++ {
		for cellY := minCellY; cellY <= maxCellY; cellY++ {
			cell := cellKey{x: cellX, y: cellY}
			w.grid[cell] = append(w.grid[cell], id)
		}
	}
}

func cellCoordF(value, cellSize float64) int {
	return int(math.Floor(value / cellSize))
}

func (w *floatWorld) narrow(a, b int32) (floatContact, bool) {
	switch {
	case !w.isRect[a] && !w.isRect[b]:
		return w.circleVsCircle(a, b)
	case w.isRect[a] && w.isRect[b]:
		return w.rectVsRect(a, b)
	case !w.isRect[a]:
		return w.circleVsRect(a, b, false)
	default:
		return w.circleVsRect(b, a, true)
	}
}

func (w *floatWorld) circleVsCircle(a, b int32) (floatContact, bool) {
	deltaX := w.posX[b] - w.posX[a]
	deltaY := w.posY[b] - w.posY[a]
	distanceSq := deltaX*deltaX + deltaY*deltaY
	radiusSum := w.extX[a] + w.extX[b]
	radiusSumSq := radiusSum * radiusSum

	if distanceSq >= radiusSumSq {
		return floatContact{}, false
	}

	if distanceSq == 0 {
		return floatContact{a: a, b: b, nx: 1, depth: radiusSum}, true
	}

	distance := math.Sqrt(distanceSq)
	return floatContact{
		a: a, b: b,
		nx:    deltaX / distance,
		ny:    deltaY / distance,
		depth: radiusSum - distance,
	}, true
}

func (w *floatWorld) circleVsRect(circle, rect int32, circleIsB bool) (floatContact, bool) {
	minX := w.posX[rect] - w.extX[rect]
	maxX := w.posX[rect] + w.extX[rect]
	minY := w.posY[rect] - w.extY[rect]
	maxY := w.posY[rect] + w.extY[rect]

	closestX := clampF(w.posX[circle], minX, maxX)
	closestY := clampF(w.posY[circle], minY, maxY)

	deltaX := closestX - w.posX[circle]
	deltaY := closestY - w.posY[circle]
	distSq := deltaX*deltaX + deltaY*deltaY

	radius := w.extX[circle]
	var nx, ny, depth float64

	if distSq == 0 {
		left := w.posX[circle] - minX
		right := maxX - w.posX[circle]
		top := w.posY[circle] - minY
		bottom := maxY - w.posY[circle]

		minDist := left
		nx, ny = 1, 0

		if right < minDist {
			minDist = right
			nx, ny = -1, 0
		}
		if top < minDist {
			minDist = top
			nx, ny = 0, 1
		}
		if bottom < minDist {
			minDist = bottom
			nx, ny = 0, -1
		}

		depth = radius + minDist
	} else {
		if distSq >= radius*radius {
			return floatContact{}, false
		}
		dist := math.Sqrt(distSq)
		nx = deltaX / dist
		ny = deltaY / dist
		depth = radius - dist
	}

	a, b := circle, rect
	if circleIsB {
		a, b = rect, circle
		nx, ny = -nx, -ny
	}
	return floatContact{a: a, b: b, nx: nx, ny: ny, depth: depth}, true
}

func (w *floatWorld) rectVsRect(a, b int32) (floatContact, bool) {
	minAX := w.posX[a] - w.extX[a]
	maxAX := w.posX[a] + w.extX[a]
	minAY := w.posY[a] - w.extY[a]
	maxAY := w.posY[a] + w.extY[a]

	minBX := w.posX[b] - w.extX[b]
	maxBX := w.posX[b] + w.extX[b]
	minBY := w.posY[b] - w.extY[b]
	maxBY := w.posY[b] + w.extY[b]

	overlapX := min(maxAX, maxBX) - max(minAX, minBX)
	if overlapX <= 0 {
		return floatContact{}, false
	}
	overlapY := min(maxAY, maxBY) - max(minAY, minBY)
	if overlapY <= 0 {
		return floatContact{}, false
	}

	if overlapX <= overlapY {
		normalX := 1.0
		if w.posX[b] < w.posX[a] {
			normalX = -1.0
		}
		return floatContact{a: a, b: b, nx: normalX, depth: overlapX}, true
	}

	normalY := 1.0
	if w.posY[b] < w.posY[a] {
		normalY = -1.0
	}
	return floatContact{a: a, b: b, ny: normalY, depth: overlapY}, true
}

// resolve mirrors the contact resolution, including the FMA shields and the
// non-finite impulse guard.
func (w *floatWorld) resolve(contacts []floatContact) {
	for _, contact := range contacts {
		invMassA, invMassB := w.invMass[contact.a], w.invMass[contact.b]
		totalInvMass := invMassA + invMassB
		if totalInvMass == 0 {
			continue
		}

		correctionA := contact.depth * (invMassA / totalInvMass)
		correctionB := contact.depth * (invMassB / totalInvMass)

		w.posX[contact.a] -= float64(contact.nx * correctionA)
		w.posY[contact.a] -= float64(contact.ny * correctionA)
		w.posX[contact.b] += float64(contact.nx * correctionB)
		w.posY[contact.b] += float64(contact.ny * correctionB)

		vRelX := w.velX[contact.a] - w.velX[contact.b]
		vRelY := w.velY[contact.a] - w.velY[contact.b]
		vRelNormal := float64(vRelX*contact.nx) + float64(vRelY*contact.ny)

		if vRelNormal > 0 {
			restitution := min(w.restitution[contact.a], w.restitution[contact.b])
			impulseMagnitude := (1 + restitution) * vRelNormal / totalInvMass
			if math.IsInf(impulseMagnitude, 0) || math.IsNaN(impulseMagnitude) {
				continue
			}
			impulseX := impulseMagnitude * contact.nx
			impulseY := impulseMagnitude * contact.ny

			w.velX[contact.a] -= float64(impulseX * invMassA)
			w.velY[contact.a] -= float64(impulseY * invMassA)
			w.velX[contact.b] += float64(impulseX * invMassB)
			w.velY[contact.b] += float64(impulseY * invMassB)
		}
	}
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

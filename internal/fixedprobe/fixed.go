package fixedprobe

import (
	"cmp"
	"slices"

	"github.com/dhannyell/fixed"
)

// fixedWorld mirrors floatWorld operation by operation over fixed.Q. The
// float64 FMA shields have no counterpart: integer arithmetic cannot fuse.
type fixedWorld struct {
	n          int
	posX, posY []fixed.Q
	velX, velY []fixed.Q
	isRect     []bool
	extX, extY []fixed.Q

	invMass     []fixed.Q
	restitution []fixed.Q

	ballistic    bool
	z, vz, omega []fixed.Q

	grid     map[cellKey][]int32
	cellSize fixed.Q
	cells    []cellKey
	pairs    []pairKey
	contacts []fixedContact
}

type fixedContact struct {
	a, b   int32
	normal fixed.Vec2
	depth  fixed.Q
}

func newFixedWorld(f *fixture) *fixedWorld {
	w := &fixedWorld{
		n:           f.n,
		posX:        slices.Clone(f.posX),
		posY:        slices.Clone(f.posY),
		velX:        slices.Clone(f.velX),
		velY:        slices.Clone(f.velY),
		isRect:      slices.Clone(f.isRect),
		extX:        slices.Clone(f.extX),
		extY:        slices.Clone(f.extY),
		invMass:     slices.Clone(f.invMass),
		restitution: slices.Clone(f.restitution),
		grid:        make(map[cellKey][]int32),
	}
	if f.z != nil {
		w.ballistic = true
		w.z = slices.Clone(f.z)
		w.vz = slices.Clone(f.vz)
		w.omega = slices.Clone(f.omega)
	}
	return w
}

func (w *fixedWorld) step(dt, cellSize fixed.Q) int {
	if w.ballistic {
		w.advanceBallistics(dt)
	}
	w.integrate(dt)
	w.bounce()
	contacts := w.detect(cellSize)
	w.resolve(contacts)
	return len(contacts)
}

func (w *fixedWorld) advanceBallistics(dt fixed.Q) {
	one := fixed.One()
	zero := fixed.Zero()

	for i := range w.n {
		vx, vy, vz := w.velX[i], w.velY[i], w.vz[i]
		omega := w.omega[i]

		speed := vx.Mul(vx).Add(vy.Mul(vy)).Add(vz.Mul(vz)).Sqrt()

		dragStep := qDragK.Mul(speed).Mul(dt)
		if dragStep.Cmp(zero) > 0 {
			dragScale := one.Div(one.Add(dragStep))
			vx = vx.Mul(dragScale)
			vy = vy.Mul(dragScale)
			vz = vz.Mul(dragScale)
		}

		if magnusRate := qMagnusK.Mul(omega); !magnusRate.Eq(zero) {
			halfStep := magnusRate.Mul(dt).Mul(fixed.Half())
			halfStepSq := halfStep.Mul(halfStep)
			denom := one.Add(halfStepSq)
			diagonal := one.Sub(halfStepSq)
			offDiagonal := halfStep.Add(halfStep)
			rotatedX := diagonal.Mul(vx).Sub(offDiagonal.Mul(vy)).Div(denom)
			rotatedY := offDiagonal.Mul(vx).Add(diagonal.Mul(vy)).Div(denom)
			vx, vy = rotatedX, rotatedY
		}

		w.velX[i] = vx
		w.velY[i] = vy
		vz = vz.Sub(qGravity.Mul(dt))

		z := w.z[i].Add(vz.Mul(dt))

		if z.Cmp(zero) <= 0 {
			z = zero
			if vz.Cmp(zero) < 0 {
				bounced := vz.Neg().Mul(qBounceDamping)
				if bounced.Less(qRestVZ) {
					vz = zero
				} else {
					vz = bounced
					w.velX[i] = w.velX[i].Mul(qBounceFriction)
					w.velY[i] = w.velY[i].Mul(qBounceFriction)
					omega = omega.Mul(qBounceFriction)
				}
			}
		}

		if z.Eq(zero) && vz.Eq(zero) {
			rf := one.Div(one.Add(qRollFriction.Mul(dt)))
			w.velX[i] = w.velX[i].Mul(rf)
			w.velY[i] = w.velY[i].Mul(rf)
		}

		w.z[i] = z
		w.vz[i] = vz
		w.omega[i] = omega.Div(one.Add(qSpinDecay.Mul(dt)))
	}
}

func (w *fixedWorld) integrate(dt fixed.Q) {
	for i := range w.n {
		w.posX[i] = w.posX[i].Add(w.velX[i].Mul(dt))
		w.posY[i] = w.posY[i].Add(w.velY[i].Mul(dt))
	}
}

func (w *fixedWorld) bounce() {
	zero := fixed.Zero()
	side := fixed.FromInt(worldSide)
	for i := range w.n {
		if w.velX[i].Eq(zero) && w.velY[i].Eq(zero) {
			continue
		}
		extX, extY := w.extX[i], w.extY[i]

		minXBound := extX
		if w.posX[i].Less(minXBound) {
			w.posX[i] = minXBound
			if w.velX[i].Cmp(zero) < 0 {
				w.velX[i] = w.velX[i].Neg()
			}
		}
		maxXBound := side.Sub(extX)
		if maxXBound.Less(w.posX[i]) {
			w.posX[i] = maxXBound
			if w.velX[i].Cmp(zero) > 0 {
				w.velX[i] = w.velX[i].Neg()
			}
		}

		minYBound := extY
		if w.posY[i].Less(minYBound) {
			w.posY[i] = minYBound
			if w.velY[i].Cmp(zero) < 0 {
				w.velY[i] = w.velY[i].Neg()
			}
		}
		maxYBound := side.Sub(extY)
		if maxYBound.Less(w.posY[i]) {
			w.posY[i] = maxYBound
			if w.velY[i].Cmp(zero) > 0 {
				w.velY[i] = w.velY[i].Neg()
			}
		}
	}
}

func (w *fixedWorld) detect(cellSize fixed.Q) []fixedContact {
	if !w.cellSize.Eq(cellSize) {
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

func (w *fixedWorld) insert(id int32, cellSize fixed.Q) {
	minCellX := cellCoordQ(w.posX[id].Sub(w.extX[id]), cellSize)
	maxCellX := cellCoordQ(w.posX[id].Add(w.extX[id]), cellSize)
	minCellY := cellCoordQ(w.posY[id].Sub(w.extY[id]), cellSize)
	maxCellY := cellCoordQ(w.posY[id].Add(w.extY[id]), cellSize)

	for cellX := minCellX; cellX <= maxCellX; cellX++ {
		for cellY := minCellY; cellY <= maxCellY; cellY++ {
			cell := cellKey{x: cellX, y: cellY}
			w.grid[cell] = append(w.grid[cell], id)
		}
	}
}

func cellCoordQ(value, cellSize fixed.Q) int {
	return value.Div(cellSize).Floor().Int()
}

func (w *fixedWorld) narrow(a, b int32) (fixedContact, bool) {
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

func (w *fixedWorld) circleVsCircle(a, b int32) (fixedContact, bool) {
	delta := fixed.Vec2{
		X: w.posX[b].Sub(w.posX[a]),
		Y: w.posY[b].Sub(w.posY[a]),
	}
	distanceSq := delta.LenSq()
	radiusSum := w.extX[a].Add(w.extX[b])
	radiusSumSq := radiusSum.Mul(radiusSum)

	if distanceSq.Cmp(radiusSumSq) >= 0 {
		return fixedContact{}, false
	}

	if distanceSq.Eq(fixed.Zero()) {
		return fixedContact{
			a: a, b: b,
			normal: fixed.Vec2{X: fixed.One()},
			depth:  radiusSum,
		}, true
	}

	distance := distanceSq.Sqrt()
	return fixedContact{
		a: a, b: b,
		normal: delta.Div(distance),
		depth:  radiusSum.Sub(distance),
	}, true
}

func (w *fixedWorld) circleVsRect(circle, rect int32, circleIsB bool) (fixedContact, bool) {
	minX := w.posX[rect].Sub(w.extX[rect])
	maxX := w.posX[rect].Add(w.extX[rect])
	minY := w.posY[rect].Sub(w.extY[rect])
	maxY := w.posY[rect].Add(w.extY[rect])

	closest := fixed.Vec2{
		X: w.posX[circle].Clamp(minX, maxX),
		Y: w.posY[circle].Clamp(minY, maxY),
	}

	delta := fixed.Vec2{
		X: closest.X.Sub(w.posX[circle]),
		Y: closest.Y.Sub(w.posY[circle]),
	}
	distSq := delta.LenSq()

	radius := w.extX[circle]
	var normal fixed.Vec2
	var depth fixed.Q

	if distSq.Eq(fixed.Zero()) {
		left := w.posX[circle].Sub(minX)
		right := maxX.Sub(w.posX[circle])
		top := w.posY[circle].Sub(minY)
		bottom := maxY.Sub(w.posY[circle])

		minDist := left
		normal = fixed.Vec2{X: fixed.One()}

		if right.Less(minDist) {
			minDist = right
			normal = fixed.Vec2{X: fixed.One().Neg()}
		}
		if top.Less(minDist) {
			minDist = top
			normal = fixed.Vec2{Y: fixed.One()}
		}
		if bottom.Less(minDist) {
			minDist = bottom
			normal = fixed.Vec2{Y: fixed.One().Neg()}
		}

		depth = radius.Add(minDist)
	} else {
		if distSq.Cmp(radius.Mul(radius)) >= 0 {
			return fixedContact{}, false
		}
		dist := distSq.Sqrt()
		normal = delta.Div(dist)
		depth = radius.Sub(dist)
	}

	a, b := circle, rect
	if circleIsB {
		a, b = rect, circle
		normal = normal.Mul(fixed.One().Neg())
	}
	return fixedContact{a: a, b: b, normal: normal, depth: depth}, true
}

func (w *fixedWorld) rectVsRect(a, b int32) (fixedContact, bool) {
	minAX := w.posX[a].Sub(w.extX[a])
	maxAX := w.posX[a].Add(w.extX[a])
	minAY := w.posY[a].Sub(w.extY[a])
	maxAY := w.posY[a].Add(w.extY[a])

	minBX := w.posX[b].Sub(w.extX[b])
	maxBX := w.posX[b].Add(w.extX[b])
	minBY := w.posY[b].Sub(w.extY[b])
	maxBY := w.posY[b].Add(w.extY[b])

	zero := fixed.Zero()

	overlapX := maxAX.Min(maxBX).Sub(minAX.Max(minBX))
	if overlapX.Cmp(zero) <= 0 {
		return fixedContact{}, false
	}
	overlapY := maxAY.Min(maxBY).Sub(minAY.Max(minBY))
	if overlapY.Cmp(zero) <= 0 {
		return fixedContact{}, false
	}

	if overlapX.Cmp(overlapY) <= 0 {
		normalX := fixed.One()
		if w.posX[b].Less(w.posX[a]) {
			normalX = normalX.Neg()
		}
		return fixedContact{a: a, b: b, normal: fixed.Vec2{X: normalX}, depth: overlapX}, true
	}

	normalY := fixed.One()
	if w.posY[b].Less(w.posY[a]) {
		normalY = normalY.Neg()
	}
	return fixedContact{a: a, b: b, normal: fixed.Vec2{Y: normalY}, depth: overlapY}, true
}

// resolve mirrors the contact resolution. Saturation replaces the float
// non-finite guard: a Q division cannot produce Inf or NaN.
func (w *fixedWorld) resolve(contacts []fixedContact) {
	zero := fixed.Zero()
	one := fixed.One()

	for _, contact := range contacts {
		invMassA, invMassB := w.invMass[contact.a], w.invMass[contact.b]
		totalInvMass := invMassA.Add(invMassB)
		if totalInvMass.Eq(zero) {
			continue
		}

		correctionA := contact.depth.Mul(invMassA.Div(totalInvMass))
		correctionB := contact.depth.Mul(invMassB.Div(totalInvMass))

		w.posX[contact.a] = w.posX[contact.a].Sub(contact.normal.X.Mul(correctionA))
		w.posY[contact.a] = w.posY[contact.a].Sub(contact.normal.Y.Mul(correctionA))
		w.posX[contact.b] = w.posX[contact.b].Add(contact.normal.X.Mul(correctionB))
		w.posY[contact.b] = w.posY[contact.b].Add(contact.normal.Y.Mul(correctionB))

		vRel := fixed.Vec2{
			X: w.velX[contact.a].Sub(w.velX[contact.b]),
			Y: w.velY[contact.a].Sub(w.velY[contact.b]),
		}
		vRelNormal := vRel.Dot(contact.normal)

		if vRelNormal.Cmp(zero) > 0 {
			restitution := w.restitution[contact.a].Min(w.restitution[contact.b])
			impulseMagnitude := one.Add(restitution).Mul(vRelNormal).Div(totalInvMass)
			impulse := contact.normal.Mul(impulseMagnitude)

			w.velX[contact.a] = w.velX[contact.a].Sub(impulse.X.Mul(invMassA))
			w.velY[contact.a] = w.velY[contact.a].Sub(impulse.Y.Mul(invMassA))
			w.velX[contact.b] = w.velX[contact.b].Add(impulse.X.Mul(invMassB))
			w.velY[contact.b] = w.velY[contact.b].Add(impulse.Y.Mul(invMassB))
		}
	}
}

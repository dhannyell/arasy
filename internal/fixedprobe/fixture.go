// Package fixedprobe measures the fixed-point kernels against float64
// mirrors of the hot simulation loops. It is a probe: it lives on a probe
// branch, mirrors the scene inner loops operation by operation, and never
// ships.
package fixedprobe

import "github.com/dhannyell/fixed"

// worldSide matches the scene benchmark bounds; stepCellSize matches the
// Step default and broadCellSize the collision benchmarks.
const (
	worldSide     = 1000
	stepCellSize  = 64
	broadCellSize = 16
)

// fixture is one deterministic scenario authored in the Q domain. The float
// mirror converts every value exactly (raw x 2^-32), so both mirrors and the
// scene world start from identical states.
type fixture struct {
	n          int
	posX, posY []fixed.Q
	velX, velY []fixed.Q
	isRect     []bool
	extX, extY []fixed.Q

	invMass     []fixed.Q
	restitution []fixed.Q

	// ballistic state; nil when the scenario has no flight
	z, vz, omega []fixed.Q
}

// Ballistic parameters shared by every flight scenario.
var (
	qGravity        = fixed.FromRatio(49, 5)  // 9.8
	qDragK          = fixed.FromRatio(1, 200) // 0.005
	qMagnusK        = fixed.FromRatio(1, 25)  // 0.04
	qBounceDamping  = fixed.FromRatio(3, 5)   // 0.6
	qBounceFriction = fixed.FromRatio(9, 10)  // 0.9
	qSpinDecay      = fixed.FromRatio(1, 2)   // 0.5
	qRollFriction   = fixed.FromInt(2)
	qRestVZ         = fixed.FromRatio(1, 2) // 0.5
)

// qf converts q to float64 exactly; every Q value fits a float64 mantissa
// scaled by a power of two.
func qf(q fixed.Q) float64 {
	return float64(q.Raw()) * 0x1p-32
}

// hash32 is a deterministic integer mix; the probe seeds all state from it
// so both mirrors read identical bits.
func hash32(i, salt uint32) uint32 {
	h := (i + salt) * 2654435761
	h ^= h >> 16
	return h * 2246822519
}

// signedUnit returns a Q in (-0.5, 0.5) from a hash.
func signedUnit(h uint32) fixed.Q {
	return fixed.FromRaw(int64(int32(h)))
}

// spreadFixture mirrors the scene benchmark placement: a square grid inside
// worldSide, radius as given, every rectEvery-th entity a rect. Velocities
// are hashed into (-30, 30) per axis so integration and bounce do real work.
func spreadFixture(count, radius, rectEvery int) *fixture {
	cols := ceilSqrt(count)
	spacing := fixed.FromRatio(worldSide, cols)
	half := spacing.Mul(fixed.Half())
	ext := fixed.FromInt(radius)
	speed := fixed.FromInt(60)

	f := &fixture{
		n:           count,
		posX:        make([]fixed.Q, count),
		posY:        make([]fixed.Q, count),
		velX:        make([]fixed.Q, count),
		velY:        make([]fixed.Q, count),
		isRect:      make([]bool, count),
		extX:        make([]fixed.Q, count),
		extY:        make([]fixed.Q, count),
		invMass:     make([]fixed.Q, count),
		restitution: make([]fixed.Q, count),
	}

	for i := range count {
		f.invMass[i] = fixed.One()
		f.restitution[i] = fixed.One()
		f.posX[i] = fixed.FromInt(i % cols).Mul(spacing).Add(half)
		f.posY[i] = fixed.FromInt(i / cols).Mul(spacing).Add(half)
		f.velX[i] = signedUnit(hash32(uint32(i), 1)).Mul(speed)
		f.velY[i] = signedUnit(hash32(uint32(i), 2)).Mul(speed)
		f.isRect[i] = rectEvery > 0 && i%rectEvery == 0
		f.extX[i] = ext
		f.extY[i] = ext
	}

	return f
}

// ballisticFixture spreads flying balls: hashed horizontal velocity, hashed
// upward launch, hashed spin. Circle colliders keep the contact path alive.
func ballisticFixture(count int) *fixture {
	f := spreadFixture(count, 1, 0)
	f.z = make([]fixed.Q, count)
	f.vz = make([]fixed.Q, count)
	f.omega = make([]fixed.Q, count)

	up := fixed.FromInt(30)
	spin := fixed.FromInt(20)
	for i := range count {
		f.vz[i] = signedUnit(hash32(uint32(i), 3)).Abs().Mul(up)
		f.omega[i] = signedUnit(hash32(uint32(i), 4)).Mul(spin)
	}
	return f
}

func ceilSqrt(value int) int {
	result := 1
	for result*result < value {
		result++
	}
	return result
}

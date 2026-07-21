package scene

import "github.com/dhannyell/frameforge/math2d"

// StepConfig configures one simulation step.
type StepConfig struct {
	Bounds  math2d.Rect
	Bounce  BounceConfig
	Physics PhysicsConfig
}

type SideMask uint8

const (
	SideLeft SideMask = 1 << iota
	SideRight
	SideTop
	SideBottom
)

type BounceConfig struct {
	// OpenSides marks bounds sides that entities may cross freely. The zero
	// value keeps all four sides closed, matching the original behavior.
	OpenSides SideMask

	// UseBodyRestitution applies an entity Body.Restitution factor to wall
	// bounces. Entities without a Body still reflect perfectly.
	UseBodyRestitution bool
}

type StepStats struct {
	Tick            uint64
	Entities        int
	BroadPhasePairs int
	Contacts        int
	GridCells       int
	Clamped         bool
}

// Step advances the world by one simulation tick.
func (w *World) Step(dt float64, config StepConfig) StepStats {
	cellSize := config.Physics.CellSize
	if cellSize <= 0 {
		cellSize = defaultCellSize
	}

	stats := StepStats{
		Tick:     w.stepTick,
		Entities: w.entityCount(),
	}

	w.contacts = w.contacts[:0]

	w.applyAcceleration(dt)
	w.advanceBallistics(dt)
	w.integrate(dt)
	w.bounceEntitiesInside(config.Bounds, config.Bounce)

	collision := w.detectCircleOverlapsGridStats(cellSize)
	w.resolveContacts(collision.contacts)

	stats.BroadPhasePairs = collision.broadPhasePairs
	stats.Contacts = len(w.contacts)
	stats.GridCells = collision.gridCells

	w.applyDamping(dt)

	w.advanceAnimations(dt)

	w.stepTick++

	return stats
}

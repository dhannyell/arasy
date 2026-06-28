package scene

import "github.com/dhannyell/frameforge/math2d"

// StepConfig configures one simulation step.
type StepConfig struct {
	Bounds  math2d.Rect
	Physics PhysicsConfig
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

	w.ApplyAcceleration(dt)
	w.Integrate(dt)
	w.BounceCirclesInside(config.Bounds)

	collision := w.detectCircleOverlapsGridStats(cellSize)
	w.resolveCircleContacts(collision.contacts)

	stats.BroadPhasePairs = collision.broadPhasePairs
	stats.Contacts = len(w.contacts)
	stats.GridCells = collision.gridCells

	w.ApplyDamping(dt)

	w.stepTick++

	return stats
}

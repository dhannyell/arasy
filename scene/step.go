package scene

import "github.com/dhannyell/frameforge/math2d"

// StepConfig configures one simulation step.
type StepConfig struct {
	Bounds  math2d.Rect
	Physics PhysicsConfig
}

// Step advances the world by one simulation tick.
func (w *World) Step(dt float64, config StepConfig) {
	cellSize := config.Physics.CellSize
	if cellSize <= 0 {
		cellSize = defaultCellSize
	}

	w.ApplyAcceleration(dt)
	w.Integrate(dt)
	w.BounceCirclesInside(config.Bounds)
	w.ResolveCircleOverlapsGrid(cellSize)
	w.ApplyDamping(dt)
}

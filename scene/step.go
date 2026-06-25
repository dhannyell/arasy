package scene

import "github.com/dhannyell/frameforge/math2d"

// StepConfig configures one simulation step.
type StepConfig struct {
	Bounds math2d.Rect
}

// Step advances the world by one simulation tick.
func (w *World) Step(dt float64, config StepConfig) {
	w.Integrate(dt)
	w.BounceCirclesInside(config.Bounds)
	w.ResolveCircleOverlaps()
}

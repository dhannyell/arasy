// Package loop provides helpers for deterministic update loops.
//
// The package is focused on fixed timestep simulation. A fixed timestep lets
// simulation code advance in stable increments, while rendering can happen at a
// different rate.
//
// Basic usage:
//
//	fixed := loop.NewFixedStep(time.Second / 60)
//
//	fixed.Add(elapsed)
//
//	for fixed.ShouldUpdate() {
//		update(fixed.Step())
//		fixed.Consume()
//	}
//
//	render(fixed.Alpha())
//
// The interpolation alpha returned by FixedStep.Alpha can be used by renderers
// to draw a state between the previous and next simulation snapshots.
package loop

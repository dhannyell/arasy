package scene

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Test-only bridge: re-exports internals to the scene_test package so the
// existing behavior tests keep exercising the private pipeline pieces.
// Not part of the public API; this file only compiles during tests.

func (w *World) DetectCircleOverlaps() []Contact { return w.detectCircleOverlaps() }

func (w *World) DetectCircleOverlapsGrid(cellSize float64) []Contact {
	return w.detectCircleOverlapsGrid(cellSize)
}

func (w *World) ResolveCircleOverlaps() { w.resolveCircleOverlaps() }

func (w *World) ResolveCircleOverlapsGrid(cellSize float64) {
	w.resolveCircleOverlapsGrid(cellSize)
}

func (w *World) ApplyAcceleration(dt float64) { w.applyAcceleration(dt) }

func (w *World) Integrate(dt float64) { w.integrate(dt) }

func (w *World) ApplyDamping(dt float64) { w.applyDamping(dt) }

func (w *World) BounceEntitiesInside(bounds math2d.Rect, cfg BounceConfig) {
	w.bounceEntitiesInside(bounds, cfg)
}

func (w *World) AdvanceAnimations(dt float64) { w.advanceAnimations(dt) }

func (w *World) DebugColliderCommands(color render.Color, strokeWidth float64, layer int) []render.Command {
	return w.debugColliderCommands(color, strokeWidth, layer)
}

func (w *World) DebugContactCommands(color render.Color, pointRadius, lineWidth, normalLength float64, layer int) []render.Command {
	return w.debugContactCommands(color, pointRadius, lineWidth, normalLength, layer)
}

func (w *World) DebugGridCommands(color render.Color, strokeWidth, dashLength float64, layer int) []render.Command {
	return w.debugGridCommands(color, strokeWidth, dashLength, layer)
}

func (w *World) DebugActiveGridCommands(fill, stroke render.Color, strokeWidth float64, layer int) []render.Command {
	return w.debugActiveGridCommands(fill, stroke, strokeWidth, layer)
}

func (w *World) DebugCandidateGridCommands(fill, stroke render.Color, strokeWidth float64, layer int) []render.Command {
	return w.debugCandidateGridCommands(fill, stroke, strokeWidth, layer)
}

func (w *World) DebugVelocityCommands(color render.Color, width, scale float64, layer int) []render.Command {
	return w.debugVelocityCommands(color, width, scale, layer)
}

func (w *World) DebugEntityIDCommands(color render.Color, size float64, offset math2d.Vec2, layer int) []render.Command {
	return w.debugEntityIDCommands(color, size, offset, layer)
}

func (w *World) DebugGridCells() []DebugGridCell { return w.debugGridCellsSnapshot() }

func DebugGridAreaCommands(bounds math2d.Rect, cellSize float64, color render.Color, strokeWidth, dashLength float64, layer int) []render.Command {
	return debugGridAreaCommands(bounds, cellSize, color, strokeWidth, dashLength, layer)
}

func (w *World) AdvanceBallisticsExport(dt float64) { w.advanceBallistics(dt) }


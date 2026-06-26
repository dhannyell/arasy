package scene

import (
	"math"
	"slices"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func (w *World) RenderCommands() []render.Command {
	ids := make([]EntityID, 0, len(w.circleSprites))
	for id := range w.circleSprites {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	commands := make([]render.Command, 0, len(ids))

	for _, id := range ids {
		sprite := w.circleSprites[id]
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		commands = append(commands, render.DrawCircle{
			Center: transform.Position,
			Radius: sprite.Radius,
			Fill:   sprite.Color,
			BaseCommand: render.BaseCommand{
				LayerValue: sprite.Layer,
			},
		})
	}

	return commands
}

func (w *World) DebugColliderCommands(color render.Color, strokeWidth float64, layer int) []render.Command {
	commands := make([]render.Command, 0, len(w.circleColliders))

	for id, collider := range w.circleColliders {
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		commands = append(commands, render.DrawCircle{
			Center:      transform.Position,
			Radius:      collider.Radius,
			Stroke:      color,
			StrokeWidth: strokeWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func (w *World) DebugContactCommands(color render.Color, pointRadius, lineWidth, normalLength float64, layer int) []render.Command {
	contacts := w.DetectCircleOverlaps()
	commands := make([]render.Command, 0, len(contacts)*2)

	for _, contact := range contacts {
		transformA, ok := w.transforms[contact.A]
		if !ok {
			continue
		}

		transformB, ok := w.transforms[contact.B]
		if !ok {
			continue
		}

		point := transformA.Position.Lerp(transformB.Position, 0.5)
		normalEnd := point.Add(contact.Normal.Mul(normalLength))

		commands = append(commands,
			render.DrawPoint{
				Center: point,
				Radius: pointRadius,
				Color:  color,
				BaseCommand: render.BaseCommand{
					LayerValue: layer,
				},
			},
			render.DrawLine{
				From:  point,
				To:    normalEnd,
				Width: lineWidth,
				Color: color,
				BaseCommand: render.BaseCommand{
					LayerValue: layer,
				},
			},
		)
	}

	return commands
}

func (w *World) DebugGridCommands(color render.Color, strokeWidth, dashLength float64, layer int) []render.Command {
	if len(w.debugGridCells) == 0 {
		return nil
	}

	commands := make([]render.Command, 0, len(w.debugGridCells)*4)

	dashStep := dashLength
	gap := dashLength * 0.5

	for _, cell := range w.debugGridCells {
		cellSize := cell.Size
		if cellSize <= 0 {
			cellSize = defaultCellSize
		}

		x0 := float64(cell.X) * cellSize
		y0 := float64(cell.Y) * cellSize
		x1 := x0 + cellSize
		y1 := y0 + cellSize

		commands = append(commands, dashedLine(x0, y0, x1, y0, strokeWidth, dashStep, gap, color, layer)...)
		commands = append(commands, dashedLine(x1, y0, x1, y1, strokeWidth, dashStep, gap, color, layer)...)
		commands = append(commands, dashedLine(x1, y1, x0, y1, strokeWidth, dashStep, gap, color, layer)...)
		commands = append(commands, dashedLine(x0, y1, x0, y0, strokeWidth, dashStep, gap, color, layer)...)
	}

	return commands
}

func (w *World) DebugActiveGridCommands(fill, stroke render.Color, strokeWidth float64, layer int) []render.Command {
	if len(w.debugGridCells) == 0 {
		return nil
	}

	commands := make([]render.Command, 0, len(w.debugGridCells))

	for _, cell := range w.debugGridCells {
		cellSize := cell.Size
		if cellSize <= 0 {
			cellSize = defaultCellSize
		}

		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{
					X: float64(cell.X) * cellSize,
					Y: float64(cell.Y) * cellSize,
				},
				math2d.Vec2{
					X: cellSize,
					Y: cellSize,
				},
			),
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: strokeWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func (w *World) DebugCandidateGridCommands(fill, stroke render.Color, strokeWidth float64, layer int) []render.Command {
	if len(w.debugGridCells) == 0 {
		return nil
	}

	commands := make([]render.Command, 0, len(w.debugGridCells))

	for _, cell := range w.debugGridCells {
		if !cell.Candidate {
			continue
		}

		cellSize := cell.Size
		if cellSize <= 0 {
			cellSize = defaultCellSize
		}

		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{
					X: float64(cell.X) * cellSize,
					Y: float64(cell.Y) * cellSize,
				},
				math2d.Vec2{
					X: cellSize,
					Y: cellSize,
				},
			),
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: strokeWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func DebugGridAreaCommands(bounds math2d.Rect, cellSize float64, color render.Color, strokeWidth, dashLength float64, layer int) []render.Command {
	if cellSize <= 0 || bounds.Width() <= 0 || bounds.Height() <= 0 {
		return nil
	}

	dashStep := dashLength
	gap := dashLength * 0.5
	commands := make([]render.Command, 0)

	minX := bounds.Min.X
	minY := bounds.Min.Y
	maxX := bounds.Max.X
	maxY := bounds.Max.Y

	for x := firstGridLine(minX, cellSize); x <= maxX; x += cellSize {
		commands = append(commands, dashedLine(x, minY, x, maxY, strokeWidth, dashStep, gap, color, layer)...)
	}

	for y := firstGridLine(minY, cellSize); y <= maxY; y += cellSize {
		commands = append(commands, dashedLine(minX, y, maxX, y, strokeWidth, dashStep, gap, color, layer)...)
	}

	commands = append(commands, dashedLine(minX, minY, maxX, minY, strokeWidth, dashStep, gap, color, layer)...)
	commands = append(commands, dashedLine(maxX, minY, maxX, maxY, strokeWidth, dashStep, gap, color, layer)...)
	commands = append(commands, dashedLine(maxX, maxY, minX, maxY, strokeWidth, dashStep, gap, color, layer)...)
	commands = append(commands, dashedLine(minX, maxY, minX, minY, strokeWidth, dashStep, gap, color, layer)...)

	return commands
}

func firstGridLine(min, cellSize float64) float64 {
	return math.Ceil(min/cellSize) * cellSize
}

func dashedLine(x1, y1, x2, y2, width, dashLen, gap float64, color render.Color, layer int) []render.Command {
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return nil
	}

	ux := dx / length
	uy := dy / length

	var commands []render.Command
	t := 0.0
	for t < length {
		segLen := math.Min(dashLen, length-t)
		commands = append(commands, render.DrawLine{
			From:  math2d.Vec2{X: x1 + ux*t, Y: y1 + uy*t},
			To:    math2d.Vec2{X: x1 + ux*(t+segLen), Y: y1 + uy*(t+segLen)},
			Width: width,
			Color: color,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
		t += segLen + gap
	}

	return commands
}

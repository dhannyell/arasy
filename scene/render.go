package scene

import "github.com/dhannyell/frameforge/render"

func (w *World) RenderCommands() []render.Command {
	commands := make([]render.Command, 0, len(w.CircleSprites))

	for id, sprite := range w.CircleSprites {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		commands = append(commands, render.DrawCircle{
			Center: transform.Position,
			Radius: sprite.Radius,
			Fill:   sprite.Color,
			Layer:  sprite.Layer,
		})
	}

	return commands
}

func (w *World) DebugColliderCommands(color render.Color, strokeWidth float64, layer int) []render.Command {
	commands := make([]render.Command, 0, len(w.CircleColliders))

	for id, collider := range w.CircleColliders {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		commands = append(commands, render.DrawCircle{
			Center:      transform.Position,
			Radius:      collider.Radius,
			Stroke:      color,
			StrokeWidth: strokeWidth,
			Layer:       layer,
		})
	}

	return commands
}

func (w *World) DebugContactCommands(color render.Color, pointRadius, lineWidth, normalLength float64, layer int) []render.Command {
	contacts := w.DetectCircleOverlaps()
	commands := make([]render.Command, 0, len(contacts)*2)

	for _, contact := range contacts {
		transformA, ok := w.Transforms[contact.A]
		if !ok {
			continue
		}

		transformB, ok := w.Transforms[contact.B]
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
				Layer:  layer,
			},
			render.DrawLine{
				From:  point,
				To:    normalEnd,
				Width: lineWidth,
				Color: color,
				Layer: layer,
			},
		)
	}

	return commands
}

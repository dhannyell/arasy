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

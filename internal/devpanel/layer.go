//go:build arr_dev

package devpanel

import "github.com/dhannyell/arasy/render"

// layerBase lifts the panel above every plausible game layer, including the
// block ui reserves for popups. Painter order is a stable sort by layer, so
// this is the whole of "the overlay draws last" — no backend needs to know
// the panel exists.
const layerBase = 1 << 20

// raiseLayer shifts a batch of panel commands into the overlay band, in
// place. It covers the primitives the ui widgets used by the panel emit; a
// widget that emitted anything else would come out at a game layer, which is
// what TestRenderStaysAboveGameLayers exists to catch.
func raiseLayer(commands []render.Command) []render.Command {
	for i, command := range commands {
		switch command := command.(type) {
		case render.DrawRect:
			command.LayerValue += layerBase
			commands[i] = command
		case render.DrawText:
			command.LayerValue += layerBase
			commands[i] = command
		case render.DrawCircle:
			command.LayerValue += layerBase
			commands[i] = command
		case render.DrawLine:
			command.LayerValue += layerBase
			commands[i] = command
		case render.DrawPolygon:
			command.LayerValue += layerBase
			commands[i] = command
		case render.DrawTriangles:
			command.LayerValue += layerBase
			commands[i] = command
		}
	}
	return commands
}

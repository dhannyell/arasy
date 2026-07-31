package devpanel

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

// debugLayerSpec is the checkbox order of the debug overlays. It is a slice
// rather than a range over the flags because the order is editorial: the ones
// a physics bug sends you to come first.
var debugLayerSpec = []struct {
	id    string
	label string
	flag  scene.DebugLayers
}{
	{"devpanel#layer-colliders", "Colliders", scene.DebugColliders},
	{"devpanel#layer-contacts", "Contacts", scene.DebugContacts},
	{"devpanel#layer-grid", "Grid", scene.DebugGrid},
	{"devpanel#layer-active-grid", "Active grid", scene.DebugActiveGrid},
	{"devpanel#layer-candidate-grid", "Candidates", scene.DebugCandidateGrid},
	{"devpanel#layer-velocity", "Velocity", scene.DebugVelocities},
	{"devpanel#layer-entity-id", "Entity ID", scene.DebugEntityIDs},
	{"devpanel#layer-render-layers", "Render layers", scene.DebugRenderLayers},
	{"devpanel#layer-input", "Input state", scene.DebugInputState},
	{"devpanel#layer-runtime", "Runtime stats", scene.DebugRuntimeStats},
}

const debugLayerColumns = 2

func layerRects(rect math2d.Rect) []math2d.Rect {
	rows := (len(debugLayerSpec) + debugLayerColumns - 1) / debugLayerColumns
	return ui.Grid(rect, debugLayerColumns, rows, rowGap, 0)
}

func (p *Panel) updateLayers(in ui.InteractScope, rect math2d.Rect, state runtime.DriveState) {
	cells := layerRects(rect)
	mask := state.DebugLayers
	changed := false

	for i, spec := range debugLayerSpec {
		next, toggled := in.CheckboxInput(spec.id, cells[i], mask.Has(spec.flag))
		if !toggled {
			continue
		}
		changed = true
		if next {
			mask |= spec.flag
		} else {
			mask &^= spec.flag
		}
	}

	// One command for the whole mask, not one per box: the driver holds the
	// set, and sending it twice in a tick would make the second overwrite the
	// first with a stale copy.
	if changed {
		p.control(runtime.ControlCommand{Kind: runtime.ControlSetDebugLayers, DebugLayers: mask})
	}
}

func (p *Panel) drawLayers(draw ui.DrawScope, rect math2d.Rect, state runtime.DriveState) {
	cells := layerRects(rect)
	for i, spec := range debugLayerSpec {
		draw.Checkbox(spec.id, cells[i], state.DebugLayers.Has(spec.flag), spec.label)
	}
}

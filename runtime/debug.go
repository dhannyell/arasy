package runtime

import (
	"fmt"
	"slices"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

type debugRenderer struct {
	driver *Driver
}

type debugContext struct {
	gameCommands []render.Command
	camera       render.Camera
}

type debugOverlay struct {
	layer scene.DebugLayers
	lines func(debugRenderer, debugContext) []string
}

var runtimeDebugOverlays = []debugOverlay{
	{layer: scene.DebugRuntimeStats, lines: debugRuntimeStatsLines},
	{layer: scene.DebugRenderLayers, lines: debugRenderLayerLines},
	{layer: scene.DebugInputState, lines: debugInputStateLines},
}

func (r debugRenderer) Commands(gameCmds []render.Command, cam render.Camera) []render.Command {
	var cmds []render.Command

	cmds = append(cmds, r.sceneCommands()...)
	cmds = append(cmds, r.runtimeCommands(debugContext{
		gameCommands: gameCmds,
		camera:       cam,
	})...)

	return cmds
}

func (r debugRenderer) sceneCommands() []render.Command {
	d := r.driver

	return d.world.DebugCommands(
		d.controls.debugLayers,
		scene.WithDebugBounds(d.debugOverlayBounds()),
		scene.WithDebugCellSize(d.cfg.Physics.CellSize),
		scene.WithDebugOverlayConfig(d.cfg.DebugOverlay),
	)
}

func (r debugRenderer) runtimeCommands(ctx debugContext) []render.Command {
	var cmds []render.Command
	pos := math2d.Vec2{X: debugHUDMargin, Y: debugHUDMargin}
	block := 0

	for _, overlay := range runtimeDebugOverlays {
		if r.driver.controls.debugLayers.Has(overlay.layer) {
			lines := overlay.lines(r, ctx)
			if len(lines) == 0 {
				continue
			}
			cmds = append(cmds, debugHUDTextCommands(lines, ctx.camera, pos, debugHUDLayer+block*100)...)
			pos.Y += debugHUDBlockHeight(len(lines))
			block++
		}
	}

	return cmds
}

func debugInputStateLines(r debugRenderer, _ debugContext) []string {
	d := r.driver
	lines := []string{"Input"}

	lastInput := d.input.LastInput()
	actions := debugInputActions(d.input.Bindings())
	for _, action := range actions {
		value := lastInput.Value(action)
		pressed := lastInput.Pressed(action)
		justPressed := lastInput.JustPressed(action)
		justReleased := lastInput.JustReleased(action)

		if value == 0 && !pressed && !justPressed && !justReleased {
			continue
		}

		lines = append(lines, fmt.Sprintf(
			"%s: %.2f pressed=%v jp=%v jr=%v",
			debugActionName(action),
			value,
			pressed,
			justPressed,
			justReleased,
		))
	}

	pointer := d.input.Pointer()
	pos := pointer.Pos()
	buttons := pointer.Buttons()
	lines = append(lines,
		fmt.Sprintf("pointer: %.0f, %.0f inside=%v", pos.X, pos.Y, pointer.Inside()),
		fmt.Sprintf("mouse: primary=%v secondary=%v middle=%v", buttons[0], buttons[1], buttons[2]),
	)

	return lines
}

func debugRuntimeStatsLines(r debugRenderer, _ debugContext) []string {
	d := r.driver
	state := d.State()

	lines := []string{
		"Runtime",
		fmt.Sprintf("tick: %d", state.Tick),
		fmt.Sprintf("paused: %v", state.Paused),
		fmt.Sprintf("speed: %.2fx", state.Speed),
		fmt.Sprintf("history: %d", state.HistoryLen),
		fmt.Sprintf("checksum: %016x", d.world.Checksum()),
	}

	if state.HistoryLen > 0 {
		lines = append(lines, fmt.Sprintf("range: %d-%d", state.HistoryStartTick, state.HistoryEndTick))
	}

	return lines
}

func debugRenderLayerLines(_ debugRenderer, ctx debugContext) []string {
	counts := make(map[int]int)

	for _, cmd := range ctx.gameCommands {
		counts[cmd.Layer()]++
	}

	layers := make([]int, 0, len(counts))
	for layer := range counts {
		layers = append(layers, layer)
	}
	slices.Sort(layers)

	lines := []string{"Render layers"}
	for _, l := range layers {
		lines = append(lines, fmt.Sprintf("%d: %d cmds", l, counts[l]))
	}

	return lines
}

func debugInputActions(bindings []input.Binding) []input.Action {
	seen := make(map[input.Action]struct{})
	actions := make([]input.Action, 0, len(bindings))

	for _, binding := range bindings {
		if !binding.Action.Valid() {
			continue
		}
		if _, ok := seen[binding.Action]; ok {
			continue
		}
		seen[binding.Action] = struct{}{}
		actions = append(actions, binding.Action)
	}

	slices.Sort(actions)
	return actions
}

func debugActionName(action input.Action) string {
	switch action {
	case input.ActionMoveX:
		return "MoveX"
	case input.ActionMoveY:
		return "MoveY"
	case input.ActionConfirm:
		return "Confirm"
	case input.ActionCancel:
		return "Cancel"
	case input.ActionPause:
		return "Pause"
	case input.ActionJump:
		return "Jump"
	default:
		return fmt.Sprintf("Action(%d)", action)
	}
}

const (
	debugHUDMargin     = 16.0
	debugHUDLineHeight = 18.0
	debugHUDBlockGap   = 12.0
	debugHUDTextSize   = 14.0
	debugHUDLayer      = 9900
)

func debugHUDBlockHeight(lines int) float64 {
	return float64(lines)*debugHUDLineHeight + debugHUDBlockGap
}

func debugHUDTextCommands(lines []string, cam render.Camera, screenPos math2d.Vec2, layer int) []render.Command {
	commands := make([]render.Command, 0, len(lines))

	for i, line := range lines {
		pos := math2d.Vec2{
			X: screenPos.X,
			Y: screenPos.Y + float64(i)*debugHUDLineHeight,
		}

		commands = append(commands, render.DrawText{
			Position: cam.ScreenToWorld(pos),
			Text:     line,
			Color:    render.RGBA(1, 1, 1, 0.95),
			Size:     cam.ScreenScalarToWorld(debugHUDTextSize),
			Align:    render.AlignLeft,
			Baseline: render.BaselineTop,
			BaseCommand: render.BaseCommand{
				LayerValue: layer + i,
			},
		})
	}

	return commands
}

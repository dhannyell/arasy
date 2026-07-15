package rhythm

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/runtime"
)

var laneActions = [...]input.Action{actionLane0, actionLane1, actionLane2, actionLane3}

func (g *Game) handleLaneInput(ctx *runtime.UpdateContext, nowMS int) {
	for lane, action := range laneActions {
		g.State.LaneDown[lane] = ctx.Input.Pressed(action)
		if ctx.Input.JustPressed(action) {
			g.State.LaneFlashUntilMS[lane] = nowMS + 110
			g.tryHitLane(ctx, lane, nowMS)
		}
		if ctx.Input.JustReleased(action) {
			g.releaseLane(ctx, lane, nowMS)
		}
	}
}

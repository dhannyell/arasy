package rhythm

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
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

func (g *Game) handlePointerInput(ctx *runtime.UpdateContext, nowMS int) {
	for _, event := range ctx.Pointer.Events {
		if event.Kind != input.EventButtonDown || event.Button != input.PointerPrimary {
			continue
		}
		g.tryHitCircle(ctx, ctx.Camera().ScreenToWorld(event.Pos), nowMS)
	}
}

func (g *Game) tryHitCircle(ctx *runtime.UpdateContext, pos math2d.Vec2, nowMS int) {
	bestID := -1
	bestAbs := missWindowMS + 1
	for i, obj := range chart {
		if obj.Kind != objectCircle || g.State.Results[i] != resultPending {
			continue
		}
		delta := nowMS - obj.TimeMS
		absolute := absInt(delta)
		if absolute > missWindowMS || pos.Sub(obj.Pos).LenSq() > obj.Radius*obj.Radius || absolute >= bestAbs {
			continue
		}
		bestID = i
		bestAbs = absolute
	}
	if bestID >= 0 {
		g.applyJudgement(ctx, bestID, judgeDelta(nowMS-chart[bestID].TimeMS))
	}
}

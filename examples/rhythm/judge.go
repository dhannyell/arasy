package rhythm

import (
	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/runtime"
)

const (
	marvelousWindowMS = 22
	perfectWindowMS   = 45
	goodWindowMS      = 90
	badWindowMS       = 135
	missWindowMS      = 160
)

func judgeDelta(deltaMS int) objectResult {
	if deltaMS < 0 {
		deltaMS = -deltaMS
	}
	switch {
	case deltaMS <= marvelousWindowMS:
		return resultMarvelous
	case deltaMS <= perfectWindowMS:
		return resultPerfect
	case deltaMS <= goodWindowMS:
		return resultGood
	case deltaMS <= badWindowMS:
		return resultBad
	default:
		return resultMiss
	}
}

func (g *Game) tryHitLane(ctx *runtime.UpdateContext, lane, nowMS int) {
	bestID := -1
	bestAbs := missWindowMS + 1
	for i, obj := range chart {
		if obj.Lane != lane || g.State.Results[i] != resultPending {
			continue
		}
		delta := nowMS - obj.TimeMS
		absolute := absInt(delta)
		if absolute <= missWindowMS && absolute < bestAbs {
			bestID = i
			bestAbs = absolute
		}
	}
	if bestID < 0 {
		return
	}

	obj := chart[bestID]
	result := judgeDelta(nowMS - obj.TimeMS)
	if isHold(obj) && result != resultMiss {
		g.State.Results[bestID] = resultHeld
		g.triggerSpecialLightning(bestID, nowMS)
		playCue(ctx, "hit_low", 0.65)
		return
	}
	g.applyJudgement(ctx, bestID, result)
}

func (g *Game) releaseLane(ctx *runtime.UpdateContext, lane, nowMS int) {
	for i, obj := range chart {
		if !isHold(obj) || obj.Lane != lane || g.State.Results[i] != resultHeld {
			continue
		}
		if nowMS < obj.EndMS-goodWindowMS {
			g.applyMiss(ctx, i)
		}
	}
}

func (g *Game) finishHolds(ctx *runtime.UpdateContext, nowMS int) {
	for i, obj := range chart {
		if !isHold(obj) || g.State.Results[i] != resultHeld {
			continue
		}
		if !ctx.Input.Pressed(laneActions[obj.Lane]) && nowMS < obj.EndMS-goodWindowMS {
			g.applyMiss(ctx, i)
			continue
		}
		if nowMS >= obj.EndMS {
			g.State.Results[i] = resultPending
			g.applyJudgement(ctx, i, resultPerfect)
		}
	}
}

func (g *Game) updateMisses(ctx *runtime.UpdateContext, nowMS int) {
	for i, obj := range chart {
		if g.State.Results[i] != resultPending {
			continue
		}
		if nowMS > obj.TimeMS+missWindowMS {
			g.applyMiss(ctx, i)
		}
	}
}

func (g *Game) applyMiss(ctx *runtime.UpdateContext, id int) {
	if g.State.Results[id] == resultHeld {
		g.State.Results[id] = resultPending
	}
	g.applyJudgement(ctx, id, resultMiss)
}

func (g *Game) applyJudgement(ctx *runtime.UpdateContext, id int, result objectResult) {
	if g.State.Results[id] != resultPending {
		return
	}
	g.State.Results[id] = result
	g.State.LastResult = result
	g.State.LastResultMS = songTimeMS(g.State, ctx.Tick)
	if result != resultMiss {
		g.triggerSpecialLightning(id, g.State.LastResultMS)
	}

	switch result {
	case resultMarvelous:
		g.State.Marvelous++
		g.addScore(id, 1000, g.State.LastResultMS)
		playCue(ctx, "hit_high", 0.9)
	case resultPerfect:
		g.State.Perfect++
		g.addScore(id, 800, g.State.LastResultMS)
		playCue(ctx, "hit_high", 0.8)
	case resultGood:
		g.State.Good++
		g.addScore(id, 500, g.State.LastResultMS)
		playCue(ctx, "hit_low", 0.7)
	case resultBad:
		g.State.Bad++
		g.State.Combo = 0
		g.State.Health -= 4
		playCue(ctx, "hit_low", 0.45)
	case resultMiss:
		g.State.Miss++
		g.State.Combo = 0
		g.State.Health -= 10
		playCue(ctx, "miss", 0.6)
	}
}

func (g *Game) addScore(id, base, nowMS int) {
	comboStarted := g.State.Combo == 0
	g.State.Combo++
	if g.State.Combo > g.State.MaxCombo {
		g.State.MaxCombo = g.State.Combo
	}
	multiplier := 1 + g.State.Combo/25
	g.State.Score += base * multiplier
	if g.State.Score > g.best {
		g.best = g.State.Score
	}
	if comboStarted {
		g.triggerLightning(chart[id].Lane, nowMS)
	}
}

func (g *Game) triggerSpecialLightning(id, nowMS int) {
	if chart[id].Kind != objectSpecial || g.State.SpecialHit[id] {
		return
	}
	g.State.SpecialHit[id] = true
	g.triggerLightning(chart[id].Lane, nowMS)
}

func (g *Game) triggerLightning(lane, nowMS int) {
	if lane < 0 || lane >= laneCount {
		return
	}
	g.State.LightningActive = true
	g.State.LightningLane = lane
	g.State.LightningUntilMS = nowMS + lightningFlashMS
}

func isHold(obj chartObject) bool {
	return obj.Kind == objectHold || (obj.Kind == objectSpecial && obj.EndMS > obj.TimeMS)
}

func chartEndMS() int {
	end := 0
	for _, obj := range chart {
		at := obj.TimeMS
		if obj.EndMS > at {
			at = obj.EndMS
		}
		if at > end {
			end = at
		}
	}
	return end
}

func playCue(ctx *runtime.UpdateContext, clip audio.ClipID, gain float64) {
	if ctx.Audio != nil {
		ctx.Audio.PlayWith(clip, audio.CueOptions{Gain: gain})
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

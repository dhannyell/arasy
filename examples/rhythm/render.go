package rhythm

import (
	"fmt"
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

const (
	worldW    = 100.0
	worldH    = 75.0
	laneCount = chartLaneCount
	hitLineY  = 62.0
	spawnY    = 18.0

	// These half-widths follow the playable black surface inside the highway
	// rails, not the full transparent texture bounds.
	roadCenterX = 50.0
	roadTopHalf = 6.7
	roadHitHalf = 17.2
)

func drawBackground(ctx *runtime.RenderContext) []render.Command {
	cmds := make([]render.Command, 0, 2)

	bgID := ctx.Assets.Texture("stage")
	hwID := ctx.Assets.Texture("highway")

	cmds = append(cmds, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     bgID,
		Dest:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldW, Y: worldH}),
	}, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 3},
		Texture:     hwID,
		Dest:        math2d.NewRect(math2d.Vec2{X: worldW - 50 - 25, Y: 15}, math2d.Vec2{X: 50, Y: 60}),
	})
	return cmds
}

func drawChart(ctx *runtime.RenderContext, state MatchState, nowMS float64) []render.Command {
	noteTex := ctx.Assets.Texture("note")
	specialTex := ctx.Assets.Texture("special")
	cmds := make([]render.Command, 0, len(chart))

	for _, obj := range chart {
		result := state.Results[obj.ID]
		if result != resultPending && !(isHold(obj) && result == resultHeld) {
			continue
		}
		if float64(renderEndMS(obj)) < nowMS-despawnAfterMS ||
			float64(obj.TimeMS) > nowMS+approachMS {
			continue
		}
		p := math2d.Clamp((float64(obj.TimeMS)-nowMS)/approachMS, 0, 1)

		y := noteY(p)
		halfRoad := roadHalfAt(y)
		laneWidth := (halfRoad * 2) / chartLaneCount
		noteHeight := laneWidth * 0.8

		noteTexture := noteTex
		if obj.Kind == objectSpecial {
			noteTexture = specialTex
		}
		if isHold(obj) {
			endP := math2d.Clamp((float64(obj.EndMS)-nowMS)/approachMS, 0, 1)
			endY := noteY(endP)
			held := state.Results[obj.ID] == resultHeld
			if endY < y {
				cmds = append(cmds, drawHoldLine(obj.Lane, endY, y, held)...)
			}
			cmds = append(cmds, drawHoldEndMarker(noteTexture, obj.Lane, endY, held))
		}

		topY := y - noteHeight/2
		bottomY := y + noteHeight/2
		topLeft, topRight := laneBoundsAtY(obj.Lane, topY, 0.10)
		bottomLeft, bottomRight := laneBoundsAtY(obj.Lane, bottomY, 0.10)
		cmds = append(cmds, render.DrawImageQuad{
			BaseCommand: render.BaseCommand{LayerValue: 11},
			Texture:     noteTexture,
			Corners: [4]math2d.Vec2{
				{X: topLeft, Y: topY},
				{X: topRight, Y: topY},
				{X: bottomRight, Y: bottomY},
				{X: bottomLeft, Y: bottomY},
			},
		})
	}
	return cmds
}

// drawHoldLine follows Clone Hero's sustain language: a clean glowing rail
// connecting the note head to its release marker, with no stretched texture.
func drawHoldLine(lane int, fromY, toY float64, held bool) []render.Command {
	from := math2d.Vec2{X: laneCenterAtY(lane, fromY), Y: fromY}
	to := math2d.Vec2{X: laneCenterAtY(lane, toY), Y: toY}
	depth := (roadHalfAt(fromY) + roadHalfAt(toY)) / (2 * roadHitHalf)
	outer := render.RGBA(1, 0.24, 0.52, 0.54)
	inner := render.RGBA(1, 0.78, 0.28, 0.95)
	if held {
		outer = render.RGBA(1, 0.14, 0.58, 0.68)
		inner = render.RGBA(1, 0.48, 0.78, 1)
	}
	return []render.Command{
		render.DrawLine{BaseCommand: render.BaseCommand{LayerValue: 10}, From: from, To: to, Width: 1.35 * depth, Color: outer},
		render.DrawLine{BaseCommand: render.BaseCommand{LayerValue: 10}, From: from, To: to, Width: 0.48 * depth, Color: inner},
	}
}

// drawActiveHoldLanes makes a successfully latched hold unambiguous: the
// lane itself remains lit until its release marker reaches the target.
func drawActiveHoldLanes(state MatchState) []render.Command {
	cmds := make([]render.Command, 0, laneCount)
	for lane := 0; lane < laneCount; lane++ {
		if !laneHasActiveHold(state, lane) {
			continue
		}
		topLeft, topRight := laneBoundsAtY(lane, spawnY, 0.16)
		bottomLeft, bottomRight := laneBoundsAtY(lane, hitLineY, 0.16)
		cmds = append(cmds, render.DrawPolygon{
			BaseCommand: render.BaseCommand{LayerValue: 9},
			Points: []math2d.Vec2{
				{X: topLeft, Y: spawnY},
				{X: topRight, Y: spawnY},
				{X: bottomRight, Y: hitLineY},
				{X: bottomLeft, Y: hitLineY},
			},
			Fill: render.RGBA(1, 0.24, 0.56, 0.16),
		})
	}
	return cmds
}

func drawHoldEndMarker(noteTex render.TextureID, lane int, y float64, held bool) render.DrawImageQuad {
	halfHeight := (roadHalfAt(y) * 2 / laneCount) * 0.20
	topY := y - halfHeight
	bottomY := y + halfHeight
	topLeft, topRight := laneBoundsAtY(lane, topY, 0.29)
	bottomLeft, bottomRight := laneBoundsAtY(lane, bottomY, 0.29)
	tint := render.RGBA(1, 0.72, 0.25, 0.92)
	if held {
		tint = render.RGBA(1, 0.35, 0.72, 1)
	}
	return render.DrawImageQuad{
		BaseCommand: render.BaseCommand{LayerValue: 11},
		Texture:     noteTex,
		Tint:        tint,
		Corners: [4]math2d.Vec2{
			{X: topLeft, Y: topY},
			{X: topRight, Y: topY},
			{X: bottomRight, Y: bottomY},
			{X: bottomLeft, Y: bottomY},
		},
	}
}

func laneHasActiveHold(state MatchState, lane int) bool {
	for i, obj := range chart {
		if isHold(obj) && obj.Lane == lane && state.Results[i] == resultHeld {
			return true
		}
	}
	return false
}

func renderEndMS(obj chartObject) int {
	if isHold(obj) {
		return obj.EndMS
	}
	return obj.TimeMS
}

func drawHitTargets(ctx *runtime.RenderContext, state MatchState, nowMS int) []render.Command {
	cmds := make([]render.Command, 0, laneCount*2)
	hitID := ctx.Assets.Texture("circle")

	for lane := 0; lane < laneCount; lane++ {
		targetHeight := (roadHitHalf * 2 / laneCount) * 0.8
		holding := laneHasActiveHold(state, lane)
		pressed := state.LaneDown[lane]
		flashing := nowMS <= state.LaneFlashUntilMS[lane]
		if pressed || flashing {
			targetHeight *= 0.82
		}
		if holding {
			targetHeight *= 0.96 + holdPulse(ctx.Time)*0.10
		}
		topY := hitLineY - targetHeight*0.5
		bottomY := hitLineY + targetHeight*0.5
		prompt := lanePromptLevel(state, nowMS, lane)
		if prompt > 0 || holding {
			centerX := (laneCenterAtY(lane, topY) + laneCenterAtY(lane, bottomY)) * 0.5
			glow := render.RGBA(1, 0.62, 0.12, 0.23)
			if prompt == 2 {
				glow = render.RGBA(1, 0.92, 0.35, 0.38)
			}
			if holding {
				glow = render.RGBA(1, 0.2, 0.6, 0.25+holdPulse(ctx.Time)*0.28)
			}
			cmds = append(cmds, render.DrawCircle{
				BaseCommand: render.BaseCommand{LayerValue: 11},
				Center:      math2d.Vec2{X: centerX, Y: hitLineY},
				Radius:      targetHeight * 0.78,
				Fill:        glow,
			})
		}

		tint := render.RGBA(1, 1, 1, 1)
		if prompt == 1 {
			tint = render.RGBA(1, 0.82, 0.38, 1)
		}
		if prompt == 2 {
			tint = render.RGBA(1, 0.95, 0.62, 1)
		}
		if flashing {
			tint = render.RGBA(1, 0.45, 0.7, 1)
		}
		if pressed {
			tint = render.RGBA(1, 0.32, 0.55, 1)
		}
		if holding {
			tint = render.RGBA(1, 0.24, 0.62, 1)
		}
		topLeft, topRight := laneBoundsAtY(lane, topY, 0.12)
		bottomLeft, bottomRight := laneBoundsAtY(lane, bottomY, 0.12)
		cmds = append(cmds, render.DrawImageQuad{
			BaseCommand: render.BaseCommand{LayerValue: 12},
			Texture:     hitID,
			Tint:        tint,
			Corners: [4]math2d.Vec2{
				{X: topLeft, Y: topY},
				{X: topRight, Y: topY},
				{X: bottomRight, Y: bottomY},
				{X: bottomLeft, Y: bottomY},
			},
		})
	}
	return cmds
}

func holdPulse(time float64) float64 {
	phase := time * 2
	phase -= math.Floor(phase)
	if phase > 0.5 {
		phase = 1 - phase
	}
	return phase * 2
}

// lanePromptLevel reports whether a lane is in the visible input window.
// The brighter level is the perfect window around the target-circle center.
func lanePromptLevel(state MatchState, nowMS, lane int) int {
	level := 0
	for i, obj := range chart {
		if obj.Lane != lane || state.Results[i] != resultPending {
			continue
		}
		delta := absInt(nowMS - obj.TimeMS)
		switch {
		case delta <= perfectWindowMS:
			return 2
		case delta <= goodWindowMS:
			level = 1
		}
	}
	return level
}

// noteY maps the remaining-time fraction (1 at spawn, 0 at the hit line) to
// a perspective-correct screen y. The road's apparent half-width is
// proportional to 1/depth, so interpolating the width harmonically keeps the
// note's world speed constant: it drifts while far away and rushes as it
// nears the hit line, matching how its sprite scale already grows.
func noteY(remaining float64) float64 {
	w := 1 / (remaining/roadTopHalf + (1-remaining)/roadHitHalf)
	return hitLineY - (hitLineY-spawnY)*(w-roadHitHalf)/(roadTopHalf-roadHitHalf)
}

func roadHalfAt(y float64) float64 {
	p := (hitLineY - y) / (hitLineY - spawnY)
	return roadHitHalf + (roadTopHalf-roadHitHalf)*p
}

func laneBoundsAtY(lane int, y, insetFraction float64) (float64, float64) {
	halfRoad := roadHalfAt(y)
	laneWidth := halfRoad * 2 / laneCount
	left := roadCenterX - halfRoad + float64(lane)*laneWidth
	inset := laneWidth * insetFraction
	return left + inset, left + laneWidth - inset
}

func laneCenterAtY(lane int, y float64) float64 {
	left, right := laneBoundsAtY(lane, y, 0)
	return (left + right) * 0.5
}

func drawHUD(state MatchState, nowMS int) []render.Command {
	cmds := []render.Command{
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 30},
			Position:    math2d.Vec2{X: 4, Y: 4},
			Text:        fmt.Sprintf("SCORE %06d", state.Score),
			Size:        2.4,
			Color:       render.RGBA(1, 1, 1, 1),
			Baseline:    render.BaselineTop,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 30},
			Position:    math2d.Vec2{X: 96, Y: 4},
			Text:        fmt.Sprintf("HP %03d", state.Health),
			Size:        2.4,
			Color:       render.RGBA(1, 0.45, 0.45, 1),
			Align:       render.AlignRight,
			Baseline:    render.BaselineTop,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 30},
			Position:    math2d.Vec2{X: 4, Y: 70},
			Text:        fmt.Sprintf("OFFSET %+dms", state.CalibrationMS),
			Size:        1.8,
			Color:       render.RGBA(0.75, 0.8, 1, 1),
			Baseline:    render.BaselineBottom,
		},
	}
	if state.Combo > 0 {
		cmds = append(cmds, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 30},
			Position:    math2d.Vec2{X: 50, Y: 10},
			Text:        fmt.Sprintf("%d COMBO", state.Combo),
			Size:        3,
			Color:       render.RGBA(1, 0.85, 0.25, 1),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineTop,
		})
	}
	if state.LastResult != resultPending && nowMS-state.LastResultMS <= 450 {
		cmds = append(cmds, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 31},
			Position:    math2d.Vec2{X: 50, Y: 53},
			Text:        resultLabel(state.LastResult),
			Size:        3.2,
			Color:       resultColor(state.LastResult),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		})
	}
	return cmds
}

func drawMenu() []render.Command {
	return []render.Command{
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 29}, Text: "RHYTHM", Size: 7, Color: render.RGBA(1, 0.3, 0.45, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 38}, Text: "PRESS SPACE TO START", Size: 2.8, Color: render.RGBA(1, 1, 1, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 43}, Text: "A  S  J  K | LEFT / RIGHT CALIBRATION", Size: 1.8, Color: render.RGBA(0.75, 0.8, 1, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
	}
}

func drawCountdown(nowMS int) []render.Command {
	seconds := (-nowMS + 999) / 1000
	return []render.Command{render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: 40},
		Position:    math2d.Vec2{X: 50, Y: 31},
		Text:        fmt.Sprintf("%d", seconds),
		Size:        9,
		Color:       render.RGBA(1, 0.85, 0.25, 1),
		Align:       render.AlignCenter,
		Baseline:    render.BaselineMiddle,
	}}
}

func drawResults(state MatchState, best int) []render.Command {
	return []render.Command{
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 24}, Text: "RESULTS", Size: 6, Color: render.RGBA(1, 0.3, 0.45, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 34}, Text: fmt.Sprintf("SCORE %06d   BEST %06d", state.Score, best), Size: 2.8, Color: render.RGBA(1, 1, 1, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 39}, Text: fmt.Sprintf("MAX COMBO %d   MISSES %d", state.MaxCombo, state.Miss), Size: 2.4, Color: render.RGBA(0.75, 0.8, 1, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
		render.DrawText{BaseCommand: render.BaseCommand{LayerValue: 40}, Position: math2d.Vec2{X: 50, Y: 46}, Text: "PRESS SPACE FOR MENU", Size: 2.2, Color: render.RGBA(1, 0.85, 0.25, 1), Align: render.AlignCenter, Baseline: render.BaselineMiddle},
	}
}

func resultLabel(result objectResult) string {
	switch result {
	case resultMarvelous:
		return "MARVELOUS"
	case resultPerfect:
		return "PERFECT"
	case resultGood:
		return "GOOD"
	case resultBad:
		return "BAD"
	case resultMiss:
		return "MISS"
	default:
		return ""
	}
}

func resultColor(result objectResult) render.Color {
	switch result {
	case resultMarvelous:
		return render.RGBA(1, 0.85, 0.25, 1)
	case resultPerfect:
		return render.RGBA(0.4, 0.9, 1, 1)
	case resultGood:
		return render.RGBA(0.55, 1, 0.55, 1)
	case resultBad:
		return render.RGBA(1, 0.55, 0.25, 1)
	default:
		return render.RGBA(1, 0.25, 0.35, 1)
	}
}

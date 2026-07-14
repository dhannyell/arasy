package twenty48

import (
	"fmt"
	"strconv"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
)

const (
	layerBoardBackground = iota
	layerCell
	layerTile
	layerTileText
	layerMovingTile
	layerMovingTileText
)

const slideFraction = 0.65

var (
	boardSize = worldSize - 2*boardMarginX
	cellSize  = (boardSize - 2*boardPad - (gridSize-1)*cellGap) / gridSize

	boardColor = render.RGBA(0.48, 0.43, 0.39, 1)
	cellColor  = render.RGBA(0.70, 0.65, 0.60, 1)
	darkText   = render.RGBA(0.35, 0.32, 0.28, 1)
	lightText  = render.RGBA(0.98, 0.97, 0.94, 1)
)

func boardBackgroundCommands() []render.Command {
	cmds := make([]render.Command, 0, 1+gridSize*gridSize)
	cmds = append(cmds, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layerBoardBackground},
		Rect:         boardRect(),
		Fill:         boardColor,
		CornerRadius: 2,
	})
	for i := 0; i < gridSize*gridSize; i++ {
		cmds = append(cmds, render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layerCell},
			Rect:         cellRect(i),
			Fill:         cellColor,
			CornerRadius: 1.2,
		})
	}
	return cmds
}

func boardRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: boardMarginX, Y: boardTop}, math2d.Vec2{X: boardSize, Y: boardSize})
}

func tileCommands(board board, animation turnAnimation, progress float64) []render.Command {
	cmds := make([]render.Command, 0, gridSize*gridSize*4)
	var hidden [boardCellCount]bool
	if animation.Active {
		if progress < slideFraction {
			for i := 0; i < animation.MotionCount; i++ {
				hidden[animation.Motions[i].To] = true
			}
		} else {
			hidden = animation.Merges
		}
		if animation.HasSpawn {
			hidden[animation.SpawnIndex] = true
		}
	}

	for i, value := range board {
		if value == 0 || hidden[i] {
			continue
		}
		cmds = append(cmds, drawTile(value, cellRect(i), 1, layerTile, layerTileText)...)
	}

	if !animation.Active {
		return cmds
	}
	if progress < slideFraction {
		slideProgress := ease.Apply(ease.InOutQuad, progress/slideFraction)
		for i := 0; i < animation.MotionCount; i++ {
			motion := animation.Motions[i]
			cmds = append(cmds, drawTile(motion.Value, movingTileRect(motion.From, motion.To, slideProgress), 1, layerMovingTile, layerMovingTileText)...)
		}
		return cmds
	}

	popProgress := math2d.Clamp((progress-slideFraction)/(1-slideFraction), 0, 1)
	mergeScale := 1.16 - 0.16*ease.Apply(ease.OutQuad, popProgress)
	for index, merged := range animation.Merges {
		if merged {
			cmds = append(cmds, drawTile(board[index], cellRect(index), mergeScale, layerMovingTile, layerMovingTileText)...)
		}
	}
	if animation.HasSpawn {
		// OutBack overshoots ~10% at its peak, so the spawn scale tops out
		// near 1.03 — a subtle pop that stays inside the cell gap.
		spawnScale := 0.72 + 0.28*ease.Apply(ease.OutBack, popProgress)
		cmds = append(cmds, drawTile(board[animation.SpawnIndex], cellRect(animation.SpawnIndex), spawnScale, layerMovingTile, layerMovingTileText)...)
	}
	return cmds
}

func drawTile(value uint64, rect math2d.Rect, scale float64, rectLayer, textLayer int) []render.Command {
	rect = rect.Scaled(scale)
	return []render.Command{
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: rectLayer},
			Rect:         rect,
			Fill:         tileColor(value),
			CornerRadius: 1.2 * scale,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: textLayer},
			Position:    rect.Center(),
			Text:        strconv.FormatUint(value, 10),
			Color:       tileTextColor(value),
			Size:        tileTextSize(value) * scale,
			Font:        demoAssets.Font("go-regular"),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		},
	}
}

func movingTileRect(from, to int, progress float64) math2d.Rect {
	fromRect := cellRect(from)
	center := fromRect.Center().Lerp(cellCenter(to), progress)
	return math2d.NewRect(center.Sub(fromRect.Size().Mul(0.5)), fromRect.Size())
}

func animationProgress(animation turnAnimation, tick uint64, alpha float64) float64 {
	if !animation.Active || tick < animation.StartTick {
		return 0
	}
	return math2d.Clamp((float64(tick-animation.StartTick)+alpha)/float64(animationTicks), 0, 1)
}

func (g *Game2048) drawHUD(ctx *runtime.RenderContext) {
	ctx.UI.FillRectIn(math2d.NewRect(math2d.Vec2{X: 30, Y: 10}, math2d.Vec2{X: 270, Y: 50}), render.RGBA(0.18, 0.16, 0.14, 0.92), ui.SpaceRef)
	ctx.UI.DrawTextIn("2048", math2d.Vec2{X: 50, Y: 35}, lightText, 32, render.AlignLeft, render.BaselineMiddle, ui.SpaceRef)
	drawScore(ctx, "SCORE", g.state.State.score, 340)
	drawScore(ctx, "BEST", g.best, 465)
	if g.state.State.phase == phasePlaying {
		ctx.UI.ButtonIn("btn_new", "New", hudNewRect(), ui.SpaceRef)
	}
	if g.state.State.undo.usable() {
		ctx.UI.ButtonIn("btn_undo", fmt.Sprintf("Undoes: %d", g.state.State.undo.Left),
			powerButtonRect(0, 1), ui.SpaceRef)
	}
}

func drawScore(ctx *runtime.RenderContext, label string, value uint64, x float64) {
	panel := math2d.NewRect(math2d.Vec2{X: x, Y: 10}, math2d.Vec2{X: 105, Y: 50})
	ctx.UI.FillRectIn(panel, render.RGBA(0.48, 0.43, 0.39, 1), ui.SpaceRef)
	ctx.UI.DrawTextIn(label, math2d.Vec2{X: panel.Center().X, Y: 26}, lightText, 11, render.AlignCenter, render.BaselineMiddle, ui.SpaceRef)
	ctx.UI.DrawTextIn(strconv.FormatUint(value, 10), math2d.Vec2{X: panel.Center().X, Y: 44}, lightText, 18, render.AlignCenter, render.BaselineMiddle, ui.SpaceRef)
}

func (g *Game2048) drawMenuOverlay(ctx *runtime.RenderContext) {
	drawOverlay(ctx, "2048", "Join the tiles and reach 2048!", "btn_play", "Play", "", "")
}

func (g *Game2048) drawGameOverOverlay(ctx *runtime.RenderContext) {
	drawOverlay(ctx, "Game over", "Final score: "+strconv.FormatUint(g.state.State.score, 10), "btn_again", "Play again", "", "")
}

func (g *Game2048) drawWonOverlay(ctx *runtime.RenderContext) {
	drawOverlay(ctx, "You win!", "You made the 2048 tile.", "btn_continue", "Continue", "btn_new_game", "New game")
}

func drawOverlay(ctx *runtime.RenderContext, title, subtitle, primaryID, primaryLabel, secondaryID, secondaryLabel string) {
	panel := overlayPanelRect()
	ctx.UI.FillRectIn(panel, render.RGBA(0.12, 0.11, 0.10, 0.94), ui.SpaceRef)
	ctx.UI.DrawTextIn(title, math2d.Vec2{X: 360, Y: 315}, lightText, 38, render.AlignCenter, render.BaselineMiddle, ui.SpaceRef)
	ctx.UI.DrawTextIn(subtitle, math2d.Vec2{X: 360, Y: 355}, lightText, 18, render.AlignCenter, render.BaselineMiddle, ui.SpaceRef)
	ctx.UI.ButtonIn(primaryID, primaryLabel, overlayPrimaryButtonRect(), ui.SpaceRef)
	if secondaryID != "" {
		ctx.UI.ButtonIn(secondaryID, secondaryLabel, overlaySecondaryButtonRect(), ui.SpaceRef)
	}
}

func cellRect(index int) math2d.Rect {
	row, col := index/gridSize, index%gridSize
	return math2d.NewRect(math2d.Vec2{
		X: boardMarginX + boardPad + float64(col)*(cellSize+cellGap),
		Y: boardTop + boardPad + float64(row)*(cellSize+cellGap),
	}, math2d.Vec2{X: cellSize, Y: cellSize})
}

func cellCenter(index int) math2d.Vec2 { return cellRect(index).Center() }

func hudNewRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 590, Y: 11}, math2d.Vec2{X: 100, Y: 48})
}

func overlayPanelRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 145, Y: 240}, math2d.Vec2{X: 430, Y: 250})
}

func overlayPrimaryButtonRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 270, Y: 390}, math2d.Vec2{X: 180, Y: 44})
}

func overlaySecondaryButtonRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 270, Y: 442}, math2d.Vec2{X: 180, Y: 34})
}

func tileColor(value uint64) render.Color {
	switch value {
	case 2:
		return render.RGBA(0.93, 0.89, 0.85, 1)
	case 4:
		return render.RGBA(0.93, 0.88, 0.78, 1)
	case 8:
		return render.RGBA(0.95, 0.69, 0.47, 1)
	case 16:
		return render.RGBA(0.96, 0.58, 0.39, 1)
	case 32:
		return render.RGBA(0.96, 0.48, 0.37, 1)
	case 64:
		return render.RGBA(0.96, 0.37, 0.23, 1)
	case 128:
		return render.RGBA(0.93, 0.81, 0.45, 1)
	case 256:
		return render.RGBA(0.93, 0.80, 0.38, 1)
	case 512:
		return render.RGBA(0.93, 0.78, 0.29, 1)
	case 1024:
		return render.RGBA(0.93, 0.77, 0.25, 1)
	case 2048:
		return render.RGBA(0.93, 0.76, 0.22, 1)
	default:
		return render.RGBA(0.24, 0.22, 0.20, 1)
	}
}

func tileTextColor(value uint64) render.Color {
	if value >= 8 {
		return lightText
	}
	return darkText
}

func tileTextSize(value uint64) float64 {
	switch {
	case value >= 1024:
		return 4.5
	case value >= 128:
		return 5
	default:
		return 6
	}
}

func powerButtonRect(index, count int) math2d.Rect {
	const width, height, gap = 110.0, 48.0, 12.0
	total := float64(count)*width + float64(count-1)*gap
	x := (viewportSize-total)/2 + float64(index)*(width+gap)
	return math2d.NewRect(math2d.Vec2{X: x, Y: powersBarY}, math2d.Vec2{X: width, Y: height})
}

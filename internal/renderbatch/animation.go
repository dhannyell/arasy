package renderbatch

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// TimeWrapPeriod is the render-clock wrap period in seconds. Time is wrapped
// before float32 conversion so the CPU reference and the WGSL shaders agree
// on flip-book frames and bounce phase.
const TimeWrapPeriod = 4096

// WrapTime wraps the render clock into [0, TimeWrapPeriod) and converts it
// to float32, mirroring the WGSL evaluation.
func WrapTime(time float64) float32 {
	wrapped := math.Mod(time, TimeWrapPeriod)
	if wrapped < 0 {
		wrapped += TimeWrapPeriod
	}
	return float32(wrapped)
}

// FractPhase returns phase wrapped into [0, 1), mirroring the WGSL
// evaluation.
func FractPhase(phase float32) float32 {
	return phase - float32(math.Floor(float64(phase)))
}

// AnimatedSrc resolves the flip-book source cell of inst at the given
// render-clock time. Src is the base cell in texture pixel coordinates; the
// selected frame advances row-major through equal cells in the same
// texture, with floor(texW/cellWidth) cells per row. texW and texH are the
// texture size in pixels, passed explicitly so the helper stays decoupled
// from asset resolution.
//
// Frames <= 1, a non-positive Rate, a zero or out-of-texture Src, or a
// frame sequence that would run outside the texture degrade to the static
// Src.
func AnimatedSrc(inst render.SpriteInstance, time float64, texW, texH int) math2d.Rect {
	src := inst.Src
	anim := inst.Animation
	if anim.Frames <= 1 || anim.Rate <= 0 || texW <= 0 || texH <= 0 {
		return src
	}
	cellW := src.Width()
	cellH := src.Height()
	if cellW <= 0 || cellH <= 0 || !IsFiniteRect(src) {
		return src
	}
	if src.Min.X < 0 || src.Min.Y < 0 || src.Max.X > float64(texW) || src.Max.Y > float64(texH) {
		return src
	}
	cellsPerRow := int(math.Floor(float64(texW) / cellW))
	rows := int(math.Floor(float64(texH) / cellH))
	if cellsPerRow < 1 || rows < 1 {
		return src
	}
	baseCol := int(src.Min.X / cellW)
	baseRow := int(src.Min.Y / cellH)
	base := baseRow*cellsPerRow + baseCol
	// The whole declared sequence must fit inside the texture; otherwise the
	// animation degrades to the static Src.
	if base < 0 || base+int(anim.Frames) > cellsPerRow*rows {
		return src
	}

	tw := WrapTime(time)
	phase := FractPhase(anim.Phase)
	step := math.Floor(float64(tw*anim.Rate + phase*float32(anim.Frames)))
	frame := 0
	if !math.IsNaN(step) && !math.IsInf(step, 0) {
		frame = int(math.Mod(step, float64(anim.Frames)))
	}

	index := base + frame
	col := index % cellsPerRow
	row := index / cellsPerRow
	// Preserve any sub-cell offset of the base Src inside its grid cell.
	offsetX := src.Min.X - float64(baseCol)*cellW
	offsetY := src.Min.Y - float64(baseRow)*cellH
	min := math2d.Vec2{
		X: float64(col)*cellW + offsetX,
		Y: float64(row)*cellH + offsetY,
	}
	return math2d.Rect{Min: min, Max: min.Add(math2d.Vec2{X: cellW, Y: cellH})}
}

// FlipBookCells validates the flip-book sequence of inst against the texture
// and returns the row-major grid it advances through: cellsPerRow and the
// base cell index of Src. ok is false when the animation degrades to the
// static Src (Frames <= 1, non-positive Rate, zero or out-of-texture Src, or
// a sequence that would run outside the texture) — the same cases
// AnimatedSrc degrades on, evaluated with the same rules so GPU backends can
// retain the base record and advance frames in the shader.
func FlipBookCells(inst render.SpriteInstance, texW, texH int) (cellsPerRow, baseIndex int, ok bool) {
	src := inst.Src
	anim := inst.Animation
	if anim.Frames <= 1 || anim.Rate <= 0 || texW <= 0 || texH <= 0 {
		return 0, 0, false
	}
	cellW := src.Width()
	cellH := src.Height()
	if cellW <= 0 || cellH <= 0 || !IsFiniteRect(src) {
		return 0, 0, false
	}
	if src.Min.X < 0 || src.Min.Y < 0 || src.Max.X > float64(texW) || src.Max.Y > float64(texH) {
		return 0, 0, false
	}
	cellsPerRow = int(math.Floor(float64(texW) / cellW))
	rows := int(math.Floor(float64(texH) / cellH))
	if cellsPerRow < 1 || rows < 1 {
		return 0, 0, false
	}
	baseCol := int(src.Min.X / cellW)
	baseRow := int(src.Min.Y / cellH)
	base := baseRow*cellsPerRow + baseCol
	if base < 0 || base+int(anim.Frames) > cellsPerRow*rows {
		return 0, 0, false
	}
	return cellsPerRow, base, true
}

// MaxVerticalInflation is VerticalInflation for a whole batch whose largest
// |Bounce| is already known: abs(maxBounce*Intensity) * (1 + abs(Pulse.Boost)),
// zero when the batch bounce is inactive. It may overestimate, never
// underestimate.
func MaxVerticalInflation(maxBounce float64, anim render.SpriteBatchAnimation) float64 {
	if anim.BounceHz <= 0 || anim.Intensity == 0 {
		return 0
	}
	return math.Abs(maxBounce*anim.Intensity) * (1 + math.Abs(anim.Pulse.Boost))
}

// BounceOffsetY returns the vertical world-space offset of inst under the
// batch animation: a non-negative sine bounce scaled by an optional
// gaussian pulse around Pulse.Center, where the instance position is its
// Dest center. It is zero when BounceHz <= 0, the instance Bounce is zero
// or the batch Intensity is zero. Intermediate operations use float32 to
// mirror the WGSL evaluation.
func BounceOffsetY(inst render.SpriteInstance, anim render.SpriteBatchAnimation) float64 {
	sprite := inst.Animation
	if anim.BounceHz <= 0 || sprite.Bounce == 0 || anim.Intensity == 0 {
		return 0
	}
	tw := WrapTime(anim.Time)
	phase := FractPhase(sprite.Phase)
	s := float32(math.Sin(float64(2 * math.Pi * (tw*float32(anim.BounceHz) + phase))))
	if s < 0 {
		s = 0
	}
	boost := float32(1)
	pulse := anim.Pulse
	if pulse.Width > 0 && pulse.Boost != 0 {
		center := inst.Dest.Center()
		dx := float32(center.X - pulse.Center.X)
		dy := float32(center.Y - pulse.Center.Y)
		d := float32(math.Sqrt(float64(dx*dx+dy*dy))) - float32(pulse.Radius)
		if d < 0 {
			d = -d
		}
		width := float32(pulse.Width)
		g := float32(math.Exp(float64(-(d * d) / (2 * width * width))))
		boost = 1 + float32(pulse.Boost)*g
	}
	return float64(-sprite.Bounce * float32(anim.Intensity) * boost * s)
}

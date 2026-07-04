package scene

import (
	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
)

func (w *World) AdvanceAnimations(dt float64) {
	w.advanceTransformTweens(dt)
	w.advanceCircleSpriteTweens(dt)
}

func (w *World) advanceTransformTweens(dt float64) {
	for id, tw := range w.transformTweens {
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		tw.Elapsed += dt
		p, finished := loopProgress(tw.Elapsed, tw.Duration, tw.Loop)
		t := ease.Apply(tw.Ease, p)

		transform.Position = tw.From.Position.Lerp(tw.To.Position, t)
		transform.Rotation = math2d.Lerp(tw.From.Rotation, tw.To.Rotation, t)
		transform.Scale = tw.From.Scale.Lerp(tw.To.Scale, t)
		w.transforms[id] = transform

		if finished {
			tw.Done = true
			if tw.RemoveOnComplete {
				delete(w.transformTweens, id)
				continue
			}
		}
		w.transformTweens[id] = tw
	}
}

func (w *World) advanceCircleSpriteTweens(dt float64) {
	for id, tw := range w.circleTweens {
		sprite, ok := w.circleSprites[id]
		if !ok {
			continue
		}

		tw.Elapsed += dt
		p, finished := loopProgress(tw.Elapsed, tw.Duration, tw.Loop)
		t := ease.Apply(tw.Ease, p)

		sprite.Radius = math2d.Lerp(tw.From.Radius, tw.To.Radius, t)
		sprite.Color = tw.From.Color.Lerp(tw.To.Color, t)
		w.circleSprites[id] = sprite

		if finished {
			tw.Done = true
			if tw.RemoveOnComplete {
				delete(w.circleTweens, id)
				continue
			}
		}
		w.circleTweens[id] = tw
	}
}

func loopProgress(elapsed, duration float64, loop LoopMode) (t float64, finished bool) {
	switch loop {
	case LoopRepeat:
		if duration <= 0 {
			return 1, false
		}
		return math2d.Frac(elapsed / duration), false
	case LoopPingPong:
		if duration <= 0 {
			return 1, false
		}
		cycle := math2d.Frac(elapsed/(2*duration)) * 2
		if cycle > 1 {
			cycle = 2 - cycle
		}
		return cycle, false
	default:
		return ease.Progress(elapsed, duration), elapsed >= duration
	}
}

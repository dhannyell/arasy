package scene

import (
	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
)

func (w *World) advanceAnimations(dt float64) {
	w.advanceTransformTweens(dt)
	w.advanceCircleSpriteTweens(dt)
}

func (w *World) advanceTransformTweens(dt float64) {
	var toRemove []EntityID

	for i, id := range w.transformTweens.ids {
		tw := w.transformTweens.dense[i]
		transform, ok := w.transforms.get(id)
		if !ok {
			continue
		}

		tw.Elapsed += dt
		p, finished := loopProgress(tw.Elapsed, tw.Duration, tw.Loop)
		t := ease.Apply(tw.Ease, p)

		transform.Position = tw.From.Position.Lerp(tw.To.Position, t)
		transform.Rotation = math2d.Lerp(tw.From.Rotation, tw.To.Rotation, t)
		transform.Scale = tw.From.Scale.Lerp(tw.To.Scale, t)
		w.transforms.set(id, transform)

		if finished {
			tw.Done = true
			if tw.RemoveOnComplete {
				toRemove = append(toRemove, id)
				continue
			}
		}
		w.transformTweens.set(id, tw)
	}

	for _, id := range toRemove {
		w.transformTweens.remove(id)
	}
}

func (w *World) advanceCircleSpriteTweens(dt float64) {
	var toRemove []EntityID

	for i, id := range w.circleTweens.ids {
		tw := w.circleTweens.dense[i]
		sprite, ok := w.circleSprites.get(id)
		if !ok {
			continue
		}

		tw.Elapsed += dt
		p, finished := loopProgress(tw.Elapsed, tw.Duration, tw.Loop)
		t := ease.Apply(tw.Ease, p)

		sprite.Radius = math2d.Lerp(tw.From.Radius, tw.To.Radius, t)
		sprite.Color = tw.From.Color.Lerp(tw.To.Color, t)
		w.circleSprites.set(id, sprite)

		if finished {
			tw.Done = true
			if tw.RemoveOnComplete {
				toRemove = append(toRemove, id)
				continue
			}
		}
		w.circleTweens.set(id, tw)
	}

	for _, id := range toRemove {
		w.circleTweens.remove(id)
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

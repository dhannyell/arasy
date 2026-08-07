package scene

import (
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
)

func (w *World) advanceAnimations(dt float64) {
	w.advanceTransformTweens(dt)
	w.advanceHopTweens(dt)
	w.advanceCircleSpriteTweens(dt)
}

func (w *World) advanceHopTweens(dt float64) {
	var toRemove []EntityID

	for i, id := range w.hopTweens.ids {
		tw := w.hopTweens.dense[i]
		transform, ok := w.transforms.get(id)
		if !ok {
			continue
		}

		tw.Elapsed += dt
		t := ease.Progress(tw.Elapsed, tw.Duration)

		transform.Position = tw.From.Lerp(tw.To, t)
		w.transforms.set(id, transform)

		// The arc is the parabola through zero at both ends: gone the moment
		// the hop lands, so the entity never rests in the air.
		height, _ := w.heights.get(id)
		height.Z = tw.Apex * 4 * t * (1 - t)
		w.heights.set(id, height)

		if t >= 1 {
			toRemove = append(toRemove, id)
			continue
		}
		w.hopTweens.set(id, tw)
	}

	for _, id := range toRemove {
		w.hopTweens.remove(id)
	}
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
		// The conversion rounds the product so the (2 - cycle) below cannot
		// fuse into an FMA; an assignment alone does not round.
		cycle := float64(math2d.Frac(elapsed/(2*duration)) * 2)
		if cycle > 1 {
			cycle = 2 - cycle
		}
		return cycle, false
	default:
		return ease.Progress(elapsed, duration), elapsed >= duration
	}
}

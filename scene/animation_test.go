package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

func TestTweenAdvancesDeterministically(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetTransformTween(id, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 3, Y: 5}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w.Step(0.25, scene.StepConfig{})

	transform, _ := w.Transform(id)
	assertVecNear(t, transform.Position, math2d.Vec2{X: 2.5})
	assertVecNear(t, transform.Scale, math2d.Vec2{X: 1.5, Y: 2})
}

func TestTweenDoneAndRemoveOnComplete(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetTransformTween(id, scene.TransformTween{
		From:             scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:               scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration:         1,
		Ease:             ease.Linear,
		Loop:             scene.LoopOnce,
		RemoveOnComplete: true,
	})

	w.Step(1, scene.StepConfig{})

	if _, ok := w.TransformTween(id); ok {
		t.Fatal("completed tween was not removed")
	}
	transform, _ := w.Transform(id)
	assertVecNear(t, transform.Position, math2d.Vec2{X: 10})
}

func TestTweenSurvivesCloneMidFlight(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetTransformTween(id, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})
	w.Step(0.25, scene.StepConfig{})

	clone := w.Clone()
	w.Step(0.25, scene.StepConfig{})
	clone.Step(0.25, scene.StepConfig{})

	transform, _ := w.Transform(id)
	cloneTransform, _ := clone.Transform(id)
	assertVecNear(t, transform.Position, cloneTransform.Position)
	if w.Checksum() != clone.Checksum() {
		t.Fatal("clone diverged after advancing the same tween")
	}
}

func TestTweenInChecksum(t *testing.T) {
	w1 := scene.NewWorld()
	id1 := spawnAt(w1, math2d.Vec2{})
	w1.SetTransformTween(id1, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w2 := scene.NewWorld()
	id2 := spawnAt(w2, math2d.Vec2{})
	w2.SetTransformTween(id2, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 20}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	if w1.Checksum() == w2.Checksum() {
		t.Fatal("checksum did not include tween state")
	}
}

func TestTweenAfterPhysics(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 100}})
	w.SetTransformTween(id, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w.Step(0.5, scene.StepConfig{})

	transform, _ := w.Transform(id)
	assertVecNear(t, transform.Position, math2d.Vec2{X: 5})
}

func TestDespawnClearsTween(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetTransformTween(id, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w.Despawn(id)

	if _, ok := w.TransformTween(id); ok {
		t.Fatal("despawn did not clear tween")
	}
}

func TestTweenTransformStartsFromCurrentTransform(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{X: 3, Y: 4})

	ok := w.TweenTransform(id, scene.Transform{
		Position: math2d.Vec2{X: 13, Y: 24},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	}, scene.TransformTweenOptions{
		Duration: 1,
		Ease:     ease.Linear,
	})

	if !ok {
		t.Fatal("TweenTransform returned false for an entity with Transform")
	}
	tween, ok := w.TransformTween(id)
	if !ok {
		t.Fatal("TweenTransform did not install a tween")
	}
	assertVecNear(t, tween.From.Position, math2d.Vec2{X: 3, Y: 4})
	assertVecNear(t, tween.To.Position, math2d.Vec2{X: 13, Y: 24})
}

func TestTweenTransformCanStopVelocity(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 5, Y: 6}})

	w.TweenTransform(id, scene.Transform{
		Position: math2d.Vec2{X: 10},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	}, scene.TransformTweenOptions{
		Duration:     1,
		Ease:         ease.Linear,
		StopVelocity: true,
	})

	velocity, ok := w.Velocity(id)
	if !ok {
		t.Fatal("StopVelocity should keep a zero Velocity component")
	}
	assertVecNear(t, velocity.Linear, math2d.Vec2{})
}

func TestTweenTransformMissingTransformReturnsFalse(t *testing.T) {
	w := scene.NewWorld()
	id := w.Spawn()

	if w.TweenTransform(id, scene.Transform{}, scene.TransformTweenOptions{}) {
		t.Fatal("TweenTransform returned true for an entity without Transform")
	}
	if _, ok := w.TransformTween(id); ok {
		t.Fatal("TweenTransform installed a tween for an entity without Transform")
	}
}

func TestSetTransformCancelsActiveTween(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetTransformTween(id, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w.SetTransform(id, scene.Transform{
		Position: math2d.Vec2{X: 50},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	w.Step(0.5, scene.StepConfig{})

	if _, ok := w.TransformTween(id); ok {
		t.Fatal("SetTransform did not cancel the active tween")
	}
	transform, _ := w.Transform(id)
	assertVecNear(t, transform.Position, math2d.Vec2{X: 50})
}

func TestRemoveTransformTweenLeavesCurrentTransform(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{})
	w.SetTransformTween(id, scene.TransformTween{
		From:     scene.Transform{Position: math2d.Vec2{}, Scale: math2d.Vec2{X: 1, Y: 1}},
		To:       scene.Transform{Position: math2d.Vec2{X: 10}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})
	w.Step(0.25, scene.StepConfig{})

	w.RemoveTransformTween(id)
	w.Step(0.25, scene.StepConfig{})

	if _, ok := w.TransformTween(id); ok {
		t.Fatal("RemoveTransformTween did not remove the tween")
	}
	transform, _ := w.Transform(id)
	assertVecNear(t, transform.Position, math2d.Vec2{X: 2.5})
}

func TestCircleSpriteTweenAdvancesDeterministically(t *testing.T) {
	w := scene.NewWorld()
	id := spawnCircleSprite(w, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
		Radius:     2,
	})
	w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
		From: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
			Radius:     2,
		},
		To: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 0.5, 0, 0.5)},
			Radius:     6,
		},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w.Step(0.25, scene.StepConfig{})

	sprite, _ := w.CircleSprite(id)
	assertNear(t, sprite.Radius, 3)
	assertColorNear(t, sprite.Color, render.RGBA(0.25, 0.125, 0, 0.875))
}

func TestCircleSpriteTweenDoneAndRemoveOnComplete(t *testing.T) {
	w := scene.NewWorld()
	id := spawnCircleSprite(w, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
		Radius:     2,
	})
	w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
		From: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
			Radius:     2,
		},
		To: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1)},
			Radius:     6,
		},
		Duration:         1,
		Ease:             ease.Linear,
		Loop:             scene.LoopOnce,
		RemoveOnComplete: true,
	})

	w.Step(1, scene.StepConfig{})

	if _, ok := w.CircleSpriteTween(id); ok {
		t.Fatal("completed circle sprite tween was not removed")
	}
	sprite, _ := w.CircleSprite(id)
	assertNear(t, sprite.Radius, 6)
	assertColorNear(t, sprite.Color, render.RGBA(1, 1, 1, 1))
}

func TestTweenCircleSpriteStartsFromCurrentSprite(t *testing.T) {
	w := scene.NewWorld()
	id := spawnCircleSprite(w, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: 3, Color: render.RGBA(0.2, 0.3, 0.4, 1)},
		Radius:     2,
	})

	ok := w.TweenCircleSprite(id, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: 3, Color: render.RGBA(1, 1, 1, 1)},
		Radius:     4,
	}, scene.CircleSpriteTweenOptions{
		Duration: 1,
		Ease:     ease.Linear,
	})

	if !ok {
		t.Fatal("TweenCircleSprite returned false for an entity with CircleSprite")
	}
	tween, ok := w.CircleSpriteTween(id)
	if !ok {
		t.Fatal("TweenCircleSprite did not install a tween")
	}
	assertNear(t, tween.From.Radius, 2)
	assertColorNear(t, tween.From.Color, render.RGBA(0.2, 0.3, 0.4, 1))
	assertNear(t, tween.To.Radius, 4)
	assertColorNear(t, tween.To.Color, render.RGBA(1, 1, 1, 1))
}

func TestTweenCircleSpriteMissingSpriteReturnsFalse(t *testing.T) {
	w := scene.NewWorld()
	id := w.Spawn()

	if w.TweenCircleSprite(id, scene.CircleSprite{}, scene.CircleSpriteTweenOptions{}) {
		t.Fatal("TweenCircleSprite returned true for an entity without CircleSprite")
	}
	if _, ok := w.CircleSpriteTween(id); ok {
		t.Fatal("TweenCircleSprite installed a tween for an entity without CircleSprite")
	}
}

func TestSetCircleSpriteCancelsActiveTween(t *testing.T) {
	w := scene.NewWorld()
	id := spawnCircleSprite(w, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
		Radius:     2,
	})
	w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
		From: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
			Radius:     2,
		},
		To: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1)},
			Radius:     6,
		},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})

	w.SetCircleSprite(id, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Color: render.RGBA(0.5, 0.5, 0.5, 1)},
		Radius:     9,
	})
	w.Step(0.5, scene.StepConfig{})

	if _, ok := w.CircleSpriteTween(id); ok {
		t.Fatal("SetCircleSprite did not cancel the active tween")
	}
	sprite, _ := w.CircleSprite(id)
	assertNear(t, sprite.Radius, 9)
	assertColorNear(t, sprite.Color, render.RGBA(0.5, 0.5, 0.5, 1))
}

func TestRemoveCircleSpriteTweenLeavesCurrentSprite(t *testing.T) {
	w := scene.NewWorld()
	id := spawnCircleSprite(w, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
		Radius:     2,
	})
	w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
		From: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(0, 0, 0, 1)},
			Radius:     2,
		},
		To: scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Color: render.RGBA(1, 1, 1, 1)},
			Radius:     6,
		},
		Duration: 1,
		Ease:     ease.Linear,
		Loop:     scene.LoopOnce,
	})
	w.Step(0.25, scene.StepConfig{})

	w.RemoveCircleSpriteTween(id)
	w.Step(0.25, scene.StepConfig{})

	if _, ok := w.CircleSpriteTween(id); ok {
		t.Fatal("RemoveCircleSpriteTween did not remove the tween")
	}
	sprite, _ := w.CircleSprite(id)
	assertNear(t, sprite.Radius, 3)
	assertColorNear(t, sprite.Color, render.RGBA(0.25, 0.25, 0.25, 1))
}

func assertVecNear(t *testing.T, got, want math2d.Vec2) {
	t.Helper()

	if !near(got.X, want.X) || !near(got.Y, want.Y) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertColorNear(t *testing.T, got, want render.Color) {
	t.Helper()

	assertNear(t, got.R, want.R)
	assertNear(t, got.G, want.G)
	assertNear(t, got.B, want.B)
	assertNear(t, got.A, want.A)
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()

	if !near(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func near(a, b float64) bool {
	const epsilon = 0.000001
	if a > b {
		return a-b <= epsilon
	}
	return b-a <= epsilon
}

func spawnCircleSprite(w *scene.World, sprite scene.CircleSprite) scene.EntityID {
	id := w.Spawn()
	w.SetCircleSprite(id, sprite)
	return id
}

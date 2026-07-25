package simfuzz

import (
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// spawnTweens reads the tween component bytes (always, so offsets stay stable)
// but only installs a tween when its base component is present, keeping
// scenarios reachable by a real game.
func spawnTweens(w *scene.World, ed EntityDef, data []byte, off *int, hasTransform, hasCircle bool, id scene.EntityID) {
	if ed.ComponentMask&CompTransformTween != 0 {
		from := readTransform(data, off)
		to := readTransform(data, off)
		duration := clampF(readF64Finite(data, off), 0, maxFuzzFloat)
		elapsed := clampF(readF64Finite(data, off), 0, duration)
		ez := ease.Kind(clampU8(readU8(data, off), uint8(ease.OutBounce)))
		loop := scene.LoopMode(clampU8(readU8(data, off), uint8(scene.LoopPingPong)))
		done := readU8(data, off) != 0
		remove := readU8(data, off) != 0
		if hasTransform {
			w.SetTransformTween(id, scene.TransformTween{
				From: from, To: to, Duration: duration, Elapsed: elapsed,
				Ease: ez, Loop: loop, Done: done, RemoveOnComplete: remove,
			})
		}
	}

	if ed.ComponentMask&CompCircleTween != 0 {
		from := readCircleSprite(data, off)
		to := readCircleSprite(data, off)
		duration := clampF(readF64Finite(data, off), 0, maxFuzzFloat)
		elapsed := clampF(readF64Finite(data, off), 0, duration)
		ez := ease.Kind(clampU8(readU8(data, off), uint8(ease.OutBounce)))
		loop := scene.LoopMode(clampU8(readU8(data, off), uint8(scene.LoopPingPong)))
		done := readU8(data, off) != 0
		remove := readU8(data, off) != 0
		if hasCircle {
			w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
				From: from, To: to, Duration: duration, Elapsed: elapsed,
				Ease: ez, Loop: loop, Done: done, RemoveOnComplete: remove,
			})
		}
	}
}

func readTransform(data []byte, off *int) scene.Transform {
	px := readF64Finite(data, off)
	py := readF64Finite(data, off)
	rot := readF64Finite(data, off)
	sx := readF64Finite(data, off)
	sy := readF64Finite(data, off)
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return scene.Transform{
		Position: math2d.Vec2{X: px, Y: py},
		Rotation: rot,
		Scale:    math2d.Vec2{X: sx, Y: sy},
	}
}

func readCircleSprite(data []byte, off *int) scene.CircleSprite {
	layer := int(readU8(data, off))
	r := clampF(readF64Finite(data, off), 0, 1)
	g := clampF(readF64Finite(data, off), 0, 1)
	b := clampF(readF64Finite(data, off), 0, 1)
	a := clampF(readF64Finite(data, off), 0, 1)
	radius := readF64Finite(data, off)
	if radius <= 0 {
		radius = 1
	}
	return scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: layer, Color: render.RGBA(r, g, b, a)},
		Radius:     radius,
	}
}

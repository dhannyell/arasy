package simfuzz

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

const maxFuzzColliderExtent = 32.0

// spawnEntity reads one EntityDef's components in mask-bit order and installs
// them on id. The read order must match compDataLen exactly.
func spawnEntity(w *scene.World, ed EntityDef, idx int, bounds math2d.Rect, id scene.EntityID) {
	off := 0
	data := ed.Data
	hasTransform := ed.ComponentMask&CompTransform != 0
	hasCircle := ed.ComponentMask&CompCircleSprite != 0

	if hasTransform {
		w.SetTransform(id, readTransform(data, &off))
	} else {
		w.SetTransform(id, scene.Transform{
			Position: spreadPos(bounds, idx),
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
	}

	if ed.ComponentMask&CompVelocity != 0 {
		w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{
			X: readF64Finite(data, &off),
			Y: readF64Finite(data, &off),
		}})
	}

	if ed.ComponentMask&CompAcceleration != 0 {
		w.SetAcceleration(id, scene.Acceleration{Linear: math2d.Vec2{
			X: readF64Finite(data, &off),
			Y: readF64Finite(data, &off),
		}})
	}

	if ed.ComponentMask&CompDamping != 0 {
		w.SetDamping(id, scene.Damping{
			Linear: clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
		})
	}

	if ed.ComponentMask&CompBody != 0 {
		w.SetBody(id, scene.Body{
			InvMass:     clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			Restitution: clampF(readF64Finite(data, &off), 0, 1),
		})
	}

	if ed.ComponentMask&CompCircleCollider != 0 {
		radius := readF64Finite(data, &off)
		if radius <= 0 {
			radius = 1
		}
		radius = min(radius, maxFuzzColliderExtent)
		w.SetCircleCollider(id, scene.CircleCollider{Radius: radius})
	}

	if ed.ComponentMask&CompRectCollider != 0 {
		hx := readF64Finite(data, &off)
		hy := readF64Finite(data, &off)
		if hx <= 0 {
			hx = 1
		}
		if hy <= 0 {
			hy = 1
		}
		hx = min(hx, maxFuzzColliderExtent)
		hy = min(hy, maxFuzzColliderExtent)
		w.SetRectCollider(id, scene.RectCollider{HalfExtends: math2d.Vec2{X: hx, Y: hy}})
	}

	if hasCircle {
		w.SetCircleSprite(id, readCircleSprite(data, &off))
	}

	if ed.ComponentMask&CompHeight != 0 {
		w.SetHeight(id, scene.Height{
			Z:  readF64Finite(data, &off),
			VZ: readF64Finite(data, &off),
		})
	}

	if ed.ComponentMask&CompSpin != 0 {
		w.SetSpin(id, scene.Spin{Omega: readF64Finite(data, &off)})
	}

	if ed.ComponentMask&CompBallistics != 0 {
		sub := clamp(int(readI32(data, &off)), 0, 16)
		w.SetBallistics(id, scene.Ballistics{
			Gravity:        readF64Finite(data, &off),
			DragK:          clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			MagnusK:        readF64Finite(data, &off),
			BounceDamping:  clampF(readF64Finite(data, &off), 0, 1),
			BounceFriction: clampF(readF64Finite(data, &off), 0, 1),
			SpinDecay:      clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			RollFriction:   clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			RestVZ:         clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			Substeps:       sub,
		})
	}

	if ed.ComponentMask&CompLocomotion != 0 {
		w.SetLocomotion(id, scene.Locomotion{
			Accel:    clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			Decel:    clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			MaxSpeed: clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
			Agility:  clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
		})
	}

	if ed.ComponentMask&CompMoveIntent != 0 {
		w.SetMoveIntent(id, scene.MoveIntent{
			Target:       math2d.Vec2{X: readF64Finite(data, &off), Y: readF64Finite(data, &off)},
			DesiredSpeed: clampF(readF64Finite(data, &off), 0, maxFuzzFloat),
		})
	}

	if ed.ComponentMask&CompImageSprite != 0 {
		w.SetImageSprite(id, scene.ImageSprite{
			Layer:   int(int32(readU32(data, &off))),
			Texture: render.TextureID(readU32(data, &off)),
			Src: math2d.Rect{
				Min: math2d.Vec2{X: readF64Finite(data, &off), Y: readF64Finite(data, &off)},
				Max: math2d.Vec2{X: readF64Finite(data, &off), Y: readF64Finite(data, &off)},
			},
			Size: math2d.Vec2{X: readF64Finite(data, &off), Y: readF64Finite(data, &off)},
			Tint: render.RGBA(
				clampF(readF64Finite(data, &off), 0, 1),
				clampF(readF64Finite(data, &off), 0, 1),
				clampF(readF64Finite(data, &off), 0, 1),
				clampF(readF64Finite(data, &off), 0, 1),
			),
		})
	}

	spawnTweens(w, ed, data, &off, hasTransform, hasCircle, id)
}

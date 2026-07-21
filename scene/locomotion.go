package scene

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
)

// applyLocomotion steers agents with Locomotion + MoveIntent toward their
// target. It is called during Step before applyAcceleration so that
// locomotion intent produces velocity, then physics (acceleration, drag,
// etc.) responds.
//
// Steering uses speed-dependent turn radius: faster agents turn more
// slowly (t = Agility*dt/speed clamped to [0,1]). Acceleration is
// asymmetric — Accel when speeding up, Decel when slowing down.
//
// Only + - * / sqrt; no transcendentals. FMA convention applied on all
// sum-of-product expressions.
func (w *World) applyLocomotion(dt float64) {
	for _, id := range w.locomotions.ids {
		l, _ := w.locomotions.get(id)
		mi, hasIntent := w.moveIntents.get(id)
		if !hasIntent {
			// No MoveIntent: still apply deceleration (coast to stop)
			vel, hasVel := w.velocities.get(id)
			if !hasVel {
				continue
			}
			speed := math.Sqrt(float64(vel.Linear.X*vel.Linear.X) + float64(vel.Linear.Y*vel.Linear.Y))
			if speed == 0 {
				continue
			}
			// Decelerate toward zero
			decelStep := l.Decel * dt
			if decelStep >= speed {
				w.velocities.set(id, Velocity{})
			} else {
				factor := (speed - decelStep) / speed
				vel.Linear.X *= factor
				vel.Linear.Y *= factor
				w.velocities.set(id, vel)
			}
			continue
		}

		pos, hasPos := w.transforms.get(id)
		if !hasPos {
			continue
		}

		to := math2d.Vec2{
			X: mi.Target.X - pos.Position.X,
			Y: mi.Target.Y - pos.Position.Y,
		}
		dist := math.Sqrt(float64(to.X*to.X) + float64(to.Y*to.Y))

		var desiredVel math2d.Vec2
		if dist < 1e-8 {
			desiredVel = math2d.Vec2{} // arrived
		} else {
			dirX := to.X / dist
			dirY := to.Y / dist
			spd := mi.DesiredSpeed
			if spd > l.MaxSpeed {
				spd = l.MaxSpeed
			}
			desiredVel = math2d.Vec2{X: dirX * spd, Y: dirY * spd}
		}

		vel, hasVel := w.velocities.get(id)
		if !hasVel {
			// No velocity yet: snap to desired
			if desiredVel.X != 0 || desiredVel.Y != 0 {
				w.velocities.set(id, Velocity{Linear: desiredVel})
			}
			continue
		}

		speed := math.Sqrt(float64(vel.Linear.X*vel.Linear.X) + float64(vel.Linear.Y*vel.Linear.Y))

		// Steer toward desired velocity
		var targetVel math2d.Vec2
		if speed > 1e-8 && (desiredVel.X != 0 || desiredVel.Y != 0) {
			// Speed-dependent turn: t = clamp(Agility*dt/speed, 0, 1)
			t := l.Agility * dt / speed
			if t > 1 {
				t = 1
			} else if t < 0 {
				t = 0
			}
			dirX := vel.Linear.X / speed
			dirY := vel.Linear.Y / speed

			desiredSpeed := math.Sqrt(float64(desiredVel.X*desiredVel.X) + float64(desiredVel.Y*desiredVel.Y))
			var desiredDirX, desiredDirY float64
			if desiredSpeed > 1e-8 {
				desiredDirX = desiredVel.X / desiredSpeed
				desiredDirY = desiredVel.Y / desiredSpeed
			}

			// Interpolate direction: dir' = normalize(dir + t*(desiredDir - dir)).
			// float64 conversions on the products stop FMA fusion (cross-arch).
			nx := dirX + float64(t*(desiredDirX-dirX))
			ny := dirY + float64(t*(desiredDirY-dirY))
			norm := math.Sqrt(float64(nx*nx) + float64(ny*ny))
			if norm > 1e-8 {
				nx /= norm
				ny /= norm
			}
			targetVel = math2d.Vec2{X: nx * desiredSpeed, Y: ny * desiredSpeed}
		} else {
			targetVel = desiredVel
		}

		// Asymmetric acceleration
		deltaX := targetVel.X - vel.Linear.X
		deltaY := targetVel.Y - vel.Linear.Y
		deltaMag := math.Sqrt(float64(deltaX*deltaX) + float64(deltaY*deltaY))
		if deltaMag == 0 {
			continue
		}

		targetSpeed := math.Sqrt(float64(targetVel.X*targetVel.X) + float64(targetVel.Y*targetVel.Y))
		rate := l.Decel
		if targetSpeed >= speed {
			rate = l.Accel
		}

		step := rate * dt
		if step >= deltaMag {
			vel.Linear = targetVel
		} else {
			factor := step / deltaMag
			vel.Linear.X += float64(deltaX * factor)
			vel.Linear.Y += float64(deltaY * factor)
		}
		w.velocities.set(id, vel)
	}
}

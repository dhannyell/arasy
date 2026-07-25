package scene

import "math"

// maxSubsteps caps Ballistics.Substeps so a bad value cannot blow up a tick.
const maxSubsteps = 32

// advanceBallistics integrates vertical flight, applies drag/Magnus forces,
// handles ground bounce/roll, and decays spin. It is called during Step
// after applyAcceleration and before integrate. It only updates velocity and
// height; horizontal position is integrated later by integrate (or, for
// continuous-collision entities, by substepFastColliders).
//
// For entities with Height + Ballistics (+ Velocity; Spin optional):
//  1. Quadratic drag on all three axes
//  2. Magnus force curves horizontal flight proportionally to spin
//  3. Gravity on Z
//  4. Semi-implicit Euler integration of Z
//  5. Ground bounce with damping + friction
//  6. Rolling friction on the ground (rational damping, no transcendentals)
//  7. Spin decay (continuous, rational)
//
// Entities with Height but no Ballistics get Z-only integration with an
// inelastic ground, letting a consumer drive flight manually. Entities with
// Ballistics + Height but no Velocity get gravity-driven Z-only integration.
func (w *World) advanceBallistics(dt float64) {
	for _, id := range w.heights.ids {
		h, _ := w.heights.get(id)

		bal, hasBal := w.ballistics.get(id)
		if !hasBal {
			// Height without Ballistics: integrate Z under its own VZ with an
			// inelastic ground. No forces are applied.
			w.heights.set(id, integrateInelasticZ(h, dt))
			continue
		}

		vel, hasVel := w.velocities.get(id)
		if !hasVel {
			// No velocity: gravity-driven Z-only with inelastic ground. The
			// explicit float64 conversions on products stop FMA fusion, keeping
			// the integrator bit-identical across architectures.
			h.VZ -= float64(bal.Gravity * dt)
			h.Z += float64(h.VZ * dt)
			if h.Z <= 0 {
				h.Z = 0
				h.VZ = 0
			}
			w.heights.set(id, h)
			continue
		}

		spin, hasSpin := w.spins.get(id)
		omega := spin.Omega

		vx, vy, vz := vel.Linear.X, vel.Linear.Y, h.VZ

		// Speed magnitude for quadratic drag.
		speed := math.Sqrt(float64(vx*vx) + float64(vy*vy) + float64(vz*vz))

		magnusRate := bal.MagnusK * omega
		dragStep := float64(bal.DragK*speed) * dt
		magnusStep := float64(magnusRate * dt)

		if dragStep >= 0 && dragStep <= 1 && magnusStep >= -1 && magnusStep <= 1 {
			// Preserve the established explicit-Euler result in its stable
			// range, including its cross-architecture FMA barriers.
			dragX := bal.DragK * speed * vx
			dragY := bal.DragK * speed * vy
			dragZ := bal.DragK * speed * vz
			magX := magnusRate * (-vy)
			magY := magnusRate * vx
			vel.Linear.X -= float64((dragX - magX) * dt)
			vel.Linear.Y -= float64((dragY - magY) * dt)
			h.VZ -= float64((dragZ + bal.Gravity) * dt)
		} else {
			// Explicit quadratic drag reverses velocity when DragK*|v|*dt > 1
			// and then grows without bound. Its closed-form speed update stays
			// finite and monotonic in that stiff regime.
			if bal.DragK > 0 && speed > 0 {
				dragScale := 1.0 / (1.0 + dragStep)
				vx *= dragScale
				vy *= dragScale
				vz *= dragScale
			}

			// A Cayley step makes stiff Magnus coupling a bounded rotation.
			// Explicit Euler injects energy and can overflow when |rate*dt|>1.
			halfStep := float64(magnusRate*dt) * 0.5
			halfStepSq := halfStep * halfStep
			denom := 1.0 + halfStepSq
			diagonal := 1.0 - halfStepSq
			offDiagonal := 2.0 * halfStep
			rotatedX := (float64(diagonal*vx) - float64(offDiagonal*vy)) / denom
			rotatedY := (float64(offDiagonal*vx) + float64(diagonal*vy)) / denom
			vx, vy = rotatedX, rotatedY

			vel.Linear.X = vx
			vel.Linear.Y = vy
			h.VZ = vz - float64(bal.Gravity*dt)
		}

		// Integrate Z.
		h.Z += float64(h.VZ * dt)

		// Ground contact. A downward hit either bounces (reflect + horizontal
		// impact friction) or, if the post-bounce speed falls below RestVZ,
		// settles onto the ground. Impact friction applies only on a real
		// bounce, so a resting ball under gravity is not clamped every tick —
		// its horizontal decay is owned by rolling friction instead. Consumers
		// should keep RestVZ > Gravity*dt*BounceDamping for settling to hold.
		if h.Z <= 0 {
			h.Z = 0
			if h.VZ < 0 {
				bounced := -h.VZ * bal.BounceDamping
				if bounced < bal.RestVZ {
					h.VZ = 0
				} else {
					h.VZ = bounced
					vel.Linear.X *= bal.BounceFriction
					vel.Linear.Y *= bal.BounceFriction
					if hasSpin {
						omega *= bal.BounceFriction
					}
				}
			}
		}

		// Rolling friction while resting on the ground (rational damping).
		if h.Z == 0 && h.VZ == 0 {
			rf := 1.0 / (1.0 + float64(bal.RollFriction*dt))
			vel.Linear.X *= rf
			vel.Linear.Y *= rf
		}

		w.velocities.set(id, vel)
		w.heights.set(id, h)

		// Spin decay: only write back when the entity actually has a Spin
		// component. Never inject Spin into an entity that lacks it.
		if hasSpin {
			omega /= 1.0 + float64(bal.SpinDecay*dt)
			w.spins.set(id, Spin{Omega: omega})
		}
	}
}

// integrateInelasticZ advances a Height under its own vertical velocity with an
// inelastic ground clamp, applying no forces.
func integrateInelasticZ(h Height, dt float64) Height {
	h.Z += float64(h.VZ * dt)
	if h.Z <= 0 {
		h.Z = 0
		h.VZ = 0
	}
	return h
}

// substepFastColliders sub-steps the horizontal motion of opt-in ballistic
// entities (Ballistics.Substeps > 1) and resolves each sub-move against static
// colliders, so a projectile faster than its own radius per tick cannot tunnel
// through thin walls. It runs after advanceBallistics and before integrate;
// entities handled here are recorded so integrate skips them.
//
// Determinism: entities and static colliders are visited in EntityID order and
// the math is the existing narrow phase (+ - * / sqrt). Cost is opt-in — slow
// entities keep the single-step integrate path and pay nothing here.
func (w *World) substepFastColliders(dt float64) {
	clear(w.ccdMoved)

	for _, id := range w.ballistics.ids {
		bal, _ := w.ballistics.get(id)
		if bal.Substeps <= 1 {
			continue
		}
		if _, hasVel := w.velocities.get(id); !hasVel {
			continue
		}
		if _, hasCollider := w.circleColliders.get(id); !hasCollider {
			continue
		}

		k := bal.Substeps
		if k > maxSubsteps {
			k = maxSubsteps
		}
		subDt := dt / float64(k)

		for range k {
			transform, ok := w.transforms.get(id)
			if !ok {
				break
			}
			vel, _ := w.velocities.get(id)
			transform.Position.X += float64(vel.Linear.X * subDt)
			transform.Position.Y += float64(vel.Linear.Y * subDt)
			w.transforms.set(id, transform)
			w.resolveAgainstStatics(id)
		}

		if w.ccdMoved == nil {
			w.ccdMoved = make(map[EntityID]struct{})
		}
		w.ccdMoved[id] = struct{}{}
	}
}

// resolveAgainstStatics resolves entity id against every static (InvMass == 0)
// collider using the existing narrow phase, one contact at a time.
func (w *World) resolveAgainstStatics(id EntityID) {
	for _, sid := range w.circleColliders.ids {
		if sid == id || w.bodyOrDefault(sid).InvMass != 0 {
			continue
		}
		if contact, ok := w.detectOverlapPair(makeEntityPair(id, sid)); ok {
			w.resolveContacts([]Contact{contact})
		}
	}
	for _, sid := range w.rectColliders.ids {
		if sid == id || w.bodyOrDefault(sid).InvMass != 0 {
			continue
		}
		// D1 precedence: a circle collider on the same entity wins.
		if _, isCircle := w.circleColliders.get(sid); isCircle {
			continue
		}
		if contact, ok := w.detectOverlapPair(makeEntityPair(id, sid)); ok {
			w.resolveContacts([]Contact{contact})
		}
	}
}

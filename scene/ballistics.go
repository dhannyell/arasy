package scene

import "math"

// advanceBallistics integrates vertical flight, applies drag/Magnus forces,
// handles ground bounce/roll, and decays spin. It is called during Step
// after applyAcceleration and before integrate.
//
// For entities with Height + Ballistics (+ Velocity; Spin optional):
//  1. Quadratic drag on all three axes
//  2. Magnus force curves horizontal flight proportionally to spin
//  3. Gravity on Z
//  4. Semi-implicit Euler integration
//  5. Ground bounce with damping + friction
//  6. Rolling friction (rational damping, no transcendentals)
//  7. Spin decay (continuous, rational)
//
// Entities with Height+Balistics but no Velocity get Z-only integration
// with inelastic ground.
func (w *World) advanceBallistics(dt float64) {
	for _, id := range w.heights.ids {
		bal, hasBal := w.ballisticsSet.get(id)
		if !hasBal {
			continue
		}
		h, _ := w.heights.get(id)

		vel, hasVel := w.velocities.get(id)
		if !hasVel {
			// No velocity: Z-only with inelastic ground
			h.VZ -= bal.Gravity * dt
			h.Z += h.VZ * dt
			if h.Z <= 0 {
				h.Z = 0
				h.VZ = 0
			}
			w.heights.set(id, h)
			continue
		}

		omega := 0.0
		if s, hasSpin := w.spins.get(id); hasSpin {
			omega = s.Omega
		}

		vx, vy, vz := vel.Linear.X, vel.Linear.Y, h.VZ

		// Speed magnitude for quadratic drag
		speed := math.Sqrt(float64(vx*vx) + float64(vy*vy) + float64(vz*vz))

		// Drag: a_d = -DragK * |v| * v
		dragX := bal.DragK * speed * vx
		dragY := bal.DragK * speed * vy
		dragZ := bal.DragK * speed * vz

		// Magnus: F = MagnusK * omega * perpendicular(v)
		magX := bal.MagnusK * omega * (-vy)
		magY := bal.MagnusK * omega * vx

		// Semi-implicit Euler: accelerate
		vel.Linear.X -= (dragX - magX) * dt
		vel.Linear.Y -= (dragY - magY) * dt
		h.VZ -= (dragZ + bal.Gravity) * dt

		// Integrate Z
		h.Z += h.VZ * dt

		// Ground contact
		if h.Z <= 0 {
			h.Z = 0
			if h.VZ < 0 {
				h.VZ = -h.VZ * bal.BounceDamping
			}
			vel.Linear.X *= bal.BounceFriction
			vel.Linear.Y *= bal.BounceFriction
			if omega != 0 {
				omega *= bal.BounceFriction
			}
			if math.Abs(h.VZ) < bal.RestVZ {
				h.VZ = 0
			}
		}

		// Rolling friction (ground, settled)
		if h.Z == 0 && h.VZ == 0 {
			rf := 1.0 / (1.0 + bal.RollFriction*dt)
			vel.Linear.X *= rf
			vel.Linear.Y *= rf
		}

		// Spin decay
		sd := 1.0 / (1.0 + bal.SpinDecay*dt)
		omega *= sd

		w.velocities.set(id, vel)
		w.heights.set(id, h)
		w.spins.set(id, Spin{Omega: omega})
	}
}

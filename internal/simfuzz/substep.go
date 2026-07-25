package simfuzz

import (
	"encoding/binary"

	"github.com/dhannyell/arasy/input"
)

type substepScenario struct {
	Script Script
	WallX  float64
}

// decodeSubstepScenario builds a scenario shaped to the continuous-collision
// (CCD) tunneling regime: a fast small projectile with Ballistics.Substeps>1
// aimed at a thin static wall. Random fuzzing almost never produces this exact
// combination (fast + thin + substeps + aligned trajectory), so this shaped
// decoder concentrates the fuzzer's mutations on the dangerous parameter space
// where CCD math (subDt = dt/k, resolveAgainstStatics) can produce NaN/Inf or
// diverge. Parameters are read from data but clamped into the regime.
//
// Layout mirrors TestBallisticsAntiTunneling: projectile at x=0 moving +X,
// wall at x=wallX with a thin X half-extent.
func decodeSubstepScenario(data []byte) substepScenario {
	// Defaults keep the scenario valid even on empty/truncated input.
	substeps := 8
	speed := 40.0
	radius := 0.11
	wallX := 5.0
	wallHalfX := 0.05
	wallHalfY := 2.0
	gravity := 0.0
	dragK := 0.0

	if len(data) >= 1 {
		substeps = clamp(int(data[0])%16, 2, 16) // always > 1 so CCD runs
	}
	if len(data) >= 2 {
		speed = 10.0 + float64(data[1])*2.0 // 10..521, always fast vs radius
	}
	if len(data) >= 3 {
		radius = 0.05 + float64(data[2])/255.0*0.5 // 0.05..0.55, small
	}
	if len(data) >= 5 {
		wallX = 2.0 + float64(binary.LittleEndian.Uint16(data[3:5]))/65535.0*18.0 // 2..20
	}
	if len(data) >= 6 {
		wallHalfX = 0.02 + float64(data[5])/255.0*0.2 // 0.02..0.22, thin
	}
	if len(data) >= 7 {
		wallHalfY = 0.5 + float64(data[6])/255.0*4.0 // 0.5..4.5
	}
	if len(data) >= 8 {
		gravity = float64(data[7]) / 255.0 * 20.0 // 0..20
	}
	if len(data) >= 9 {
		dragK = float64(data[8]) / 255.0 // 0..1
	}

	// Projectile: Transform | Velocity | Body | CircleCollider | Height |
	// Ballistics. The packed field order must match spawnEntity's mask-bit
	// order, not the order used when the mask expression is written.
	projMask := CompTransform | CompVelocity | CompCircleCollider | CompBody | CompHeight | CompBallistics
	projData := concatBytes(
		packF64(0), packF64(0), packF64(0), packF64(1), packF64(1), // Transform: x,y,rot,sx,sy
		packF64(speed), packF64(0), // Velocity: vx, vy
		packF64(1), packF64(0), // Body: InvMass=1 (dynamic), Restitution=0
		packF64(radius),        // CircleCollider
		packF64(0), packF64(0), // Height: Z=0, VZ=0
		packI32(int32(substeps)),                     // Ballistics.Substeps (read first)
		packF64(gravity), packF64(dragK), packF64(0), // Gravity, DragK, MagnusK
		packF64(0.5), packF64(0.5), packF64(0), packF64(0), // BounceDamping, BounceFriction, SpinDecay, RollFriction
		packF64(0), // RestVZ
	)

	// Wall: Transform | RectCollider | Body (static). Data order: Transform(40),
	// Body(16), RectCollider(16) = 72 bytes. (Body bit is lower than RectCollider.)
	wallMask := CompTransform | CompRectCollider | CompBody
	wallData := concatBytes(
		packF64(wallX), packF64(0), packF64(0), packF64(1), packF64(1), // Transform
		packF64(0), packF64(0), // Body: InvMass=0 (static), Restitution=0
		packF64(wallHalfX), packF64(wallHalfY), // RectCollider half-extents
	)

	return substepScenario{
		WallX: wallX,
		Script: Script{
			Seed:     seedFromBytes(data),
			MaxTicks: 60,
			Entities: []EntityDef{
				{ComponentMask: projMask, Data: projData},
				{ComponentMask: wallMask, Data: wallData},
			},
			Events: nil,
		},
	}
}

// decodeForkEvents reads a second, bound input event stream from a different
// region of data than decodeEvents, for P12: the "new input" applied after a
// scrub to fork the timeline. It reads from the middle of the input so it does
// not collide with the base event log decoded from the tail by DecodeScript.
func decodeForkEvents(data []byte, maxTicks int) []input.Event {
	if maxTicks < 3 {
		return nil
	}

	k := maxTicks / 2
	// Always create a fresh edge in the forked future. Arrow keys affect the
	// fuzz game's velocity path without triggering its despawn behavior.
	_ = data
	source := input.Keyboard(input.KeyArrowRight)
	return []input.Event{
		{Tick: uint64(k), Source: source, Kind: input.EventButtonUp},
		{Tick: uint64(k + 1), Source: source, Kind: input.EventButtonDown},
	}
}

package simfuzz

import "testing"

// seedBytes builds a compact fuzz input with zeroed component data and no
// events. addSimulationSeeds complements it with degenerate, extreme,
// full-vocabulary, and golden-replay-derived scenarios.
func seedBytes(ticks int, masks ...uint32) []byte {
	ticks = clamp(ticks, 1, MaxTicks)
	buf := []byte{1, 0, byte(len(masks)), byte(ticks), byte(ticks >> 8)}
	for _, mask := range masks {
		buf = append(buf, byte(mask), byte(mask>>8), byte(mask>>16), byte(mask>>24))
		buf = append(buf, make([]byte, compDataLen(mask))...)
	}
	return buf
}

// FuzzDeterminism (P1): two independent drivers, identical checksum per tick.
func FuzzDeterminism(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompBody|CompBallistics|CompHeight|CompSpin,
		CompTransform|CompLocomotion|CompMoveIntent))
	f.Fuzz(func(t *testing.T, data []byte) { checkDeterminism(t, data) })
}

// FuzzReplayFidelity (P2): record -> VerifyRecording -> checksum == live run.
func FuzzReplayFidelity(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider|CompCircleSprite,
		CompTransform|CompBody|CompBallistics|CompHeight|CompSpin|CompRectCollider))
	f.Fuzz(func(t *testing.T, data []byte) {
		checkReplayFidelity(t, data)
		checkBreakoutReplayFidelity(t, data)
	})
}

// FuzzRewindConvergence (P3): advance to N, rewind to K<N, re-advance to N.
func FuzzRewindConvergence(f *testing.F) {
	addSimulationSeeds(f, seedBytes(90,
		CompTransform|CompVelocity|CompDamping|CompCircleCollider,
		CompTransform|CompAcceleration|CompBody))
	f.Fuzz(func(t *testing.T, data []byte) { checkRewindConvergence(t, data) })
}

// FuzzNumericSanity (P4): no NaN/Inf in any component after any tick.
func FuzzNumericSanity(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompBody|CompBallistics|CompHeight|CompSpin,
		CompTransform|CompLocomotion|CompMoveIntent|CompCircleCollider))
	f.Fuzz(func(t *testing.T, data []byte) { checkNumericSanity(t, data) })
}

// FuzzRecordingCodec (P5): Recording -> bytes -> Recording round-trip.
func FuzzRecordingCodec(f *testing.F) {
	addRecordingSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) { checkRecordingCodec(t, data) })
}

// FuzzNoPanic (P6): arbitrary input never panics the engine.
func FuzzNoPanic(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompLocomotion|CompMoveIntent))
	f.Fuzz(func(t *testing.T, data []byte) { checkNoPanic(t, data) })
}

// FuzzResetReproducibility (P7): ControlReset mid-run rebuilds the world
// deterministically; post-reset per-tick checksums match a fresh run.
func FuzzResetReproducibility(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompBody|CompBallistics|CompHeight|CompSpin))
	f.Fuzz(func(t *testing.T, data []byte) { checkResetReproducibility(t, data) })
}

// FuzzScrubBidirectional (P8): repeated scrub jumps land on the exact original
// checksum each time, exercising keyframe archive thinning.
func FuzzScrubBidirectional(f *testing.F) {
	addSimulationSeeds(f, seedBytes(90,
		CompTransform|CompVelocity|CompDamping|CompCircleCollider,
		CompTransform|CompAcceleration|CompBody,
		CompTransform|CompLocomotion|CompMoveIntent))
	f.Fuzz(func(t *testing.T, data []byte) { checkScrubBidirectional(t, data) })
}

// FuzzSpeedIndependence (P9): ControlSetSpeed changes tick pacing, never
// per-tick state; different speed histories give identical checksums.
func FuzzSpeedIndependence(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompAcceleration|CompBody))
	f.Fuzz(func(t *testing.T, data []byte) { checkSpeedIndependence(t, data) })
}

// FuzzDespawnMidRun (P10): input-triggered World.Despawn stays deterministic,
// NaN-free, and converges across a rewind.
func FuzzDespawnMidRun(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompBody|CompBallistics|CompHeight|CompSpin,
		CompTransform|CompLocomotion|CompMoveIntent))
	f.Fuzz(func(t *testing.T, data []byte) { checkDespawnMidRun(t, data) })
}

// FuzzSubstepTunneling (P11): a shaped fast-projectile + thin-wall + substeps
// scenario drives the CCD path; it must stay finite and deterministic.
func FuzzSubstepTunneling(f *testing.F) {
	f.Add([]byte{8, 20, 11, 0x05, 0x00, 5, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) { checkSubstepTunneling(t, data) })
}

// FuzzForkAfterScrubDeterminism (P12): scrubbing back and applying new input
// forks the timeline; the forked future must equal a clean base-then-new run.
func FuzzForkAfterScrubDeterminism(f *testing.F) {
	addSimulationSeeds(f, seedBytes(60,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompAcceleration|CompBody))
	f.Fuzz(func(t *testing.T, data []byte) { checkForkAfterScrubDeterminism(t, data) })
}

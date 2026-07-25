// Package simfuzz provides deterministic fuzz targets for the simulation
// pipeline. Each target decodes a Script from the fuzzer bytes, drives a
// dedicated runtime.Game (the "fuzz game") through a real runtime.Driver, and
// checks a determinism property of the engine.
//
// The decoder is total: every byte sequence produces a valid Script without
// panicking or returning an error. Missing bytes default to zero. The fuzz
// game spawns entities with the full scene component vocabulary and reacts to
// a monotonic input event log decoded from the same bytes, so the fuzzer
// exercises the input -> Driver -> World pipeline, not just World.Step.
//
// Properties:
//
//   - P1 FuzzDeterminism         two independent drivers, identical checksum per tick
//   - P2 FuzzReplayFidelity      record -> VerifyRecording -> checksum == live run
//   - P3 FuzzRewindConvergence   advance to N, rewind to K<N, re-advance to N: same checksum
//   - P4 FuzzNumericSanity       no NaN/Inf in any component after any tick
//   - P5 FuzzRecordingCodec      Recording -> bytes -> Recording round-trip
//   - P6 FuzzNoPanic             arbitrary input never panics the engine
//   - P7 FuzzResetReproducibility ControlReset mid-run rebuilds the world deterministically
//   - P8 FuzzScrubBidirectional  repeated scrub jumps land on the original checksum
//   - P9 FuzzSpeedIndependence   ControlSetSpeed changes pacing, never per-tick state
//   - P10 FuzzDespawnMidRun      input-driven despawn stays deterministic across rewind
//   - P11 FuzzSubstepTunneling   shaped CCD projectiles do not cross thin static walls
//   - P12 FuzzForkAfterScrubDeterminism a forked future matches a clean branched run
//
// Run the seed corpus for free on every `go test ./...`; fuzz under demand with
// `go test ./internal/simfuzz -fuzz FuzzDeterminism -fuzztime 120s`. Assertion
// failures write a native Go corpus entry and an ARRP timeline under
// testdata/repro/<target>/; the Go fuzz runner also persists panic inputs.
package simfuzz

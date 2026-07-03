// Package runtime is the engine-shared integration layer between a game and
// the platform that drives it.
//
// Games implement Game once: Setup returns portable configuration, Init builds
// a fresh scene.World, Update reads input and drives entities for one
// fixed-timestep simulation tick, and Render returns backend-agnostic
// render.Command values. Driver steps the World automatically after Update,
// unless the game calls UpdateContext.HoldSimulation; games can implement
// PostStepper to react to same-tick contacts.
//
// Driver owns the portable runtime state used by every platform target. It
// applies raw input.Event values through the input mapper, advances the game
// according to play/pause/step/speed controls, records optional snapshots for
// rewind/time-travel, and produces complete Frames for render backends. Driver
// intentionally contains no transport, windowing, HTTP, canvas, or raster
// details; each platform serializes access and adapts its native input/output
// to ApplyInput, Control, AdvanceTime, Step, and Frame.
//
// The package sits above the core engine packages and below platform packages:
//
//	cmd/* -> platform/* -> runtime -> scene/render/input -> math2d
//
// This keeps game code portable across current and future targets such as the
// web canvas runner, offline raster output, native windows, WASM, or GPU-backed
// renderers.
package runtime

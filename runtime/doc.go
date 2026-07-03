// Package runtime connects games to platform hosts.
//
// A Game provides Setup, Init, Update and Render. Driver owns fixed ticks,
// input mapping, optional rewind snapshots and frame production. It steps the
// World after Update unless the game calls UpdateContext.HoldSimulation.
//
// Runtime has no transport, windowing, HTTP, canvas or raster knowledge.
// Platforms adapt native input/output to ApplyInput, Control, AdvanceTime,
// Step and Frame.
//
// The package sits above the core engine packages and below platform packages:
//
//	cmd/* -> platform/* -> runtime -> scene/render/input -> math2d
//
// This keeps game code portable across web canvas, offline raster output and
// future targets.
package runtime

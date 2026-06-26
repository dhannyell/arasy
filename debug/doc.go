// Package debug contains development-time tools for inspecting and controlling
// engine state.
//
// The package is intentionally separate from gameplay systems. Its recorder is
// snapshot-based, which makes it useful for local debugging, time travel in
// demos, and deterministic test inspection. It is not intended to be the final
// format for gameplay replay, save files, or networking.
package debug

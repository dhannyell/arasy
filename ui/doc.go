// Package ui provides a small immediate-mode UI layer built on top of render
// commands and deterministic pointer input.
//
// The package does not talk to platform APIs directly. A runtime.Driver builds a
// UI value for Update with the pointer frame for the current simulation tick,
// and another UI value for Render with the current cursor position. Games call
// interaction queries such as Clicked during Update, then draw widgets such as
// Button, Label, Panel and ProgressBar during Render.
//
// UI geometry is expressed in viewport pixels. The package converts that
// screen-space geometry to world-space render commands through the active
// camera, so the existing render pipeline can draw UI without a separate
// backend path. UI commands use LayerBase and above so they render over normal
// game commands.
package ui

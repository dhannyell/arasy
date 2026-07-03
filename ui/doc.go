// Package ui provides immediate-mode widgets backed by render commands.
//
// The package does not talk to platform APIs directly. A runtime.Driver builds
// one UI value for Update interaction queries and another for Render drawing.
//
// UI geometry is expressed in viewport pixels. It is converted to render
// commands through the active camera and drawn on LayerBase or above.
package ui

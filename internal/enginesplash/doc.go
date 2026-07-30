// Package enginesplash provides the engine's compiled boot animation and its
// supporting particle assets.
package enginesplash

import "github.com/dhannyell/arasy/render"

const (
	// BackgroundLayer keeps the host-painted backdrop behind all
	// compiled splash content.
	BackgroundLayer = -2
	engineSkyLayer  = -1
)

// MinTime matches the compiled animation duration.
const MinTime = 2.0

// Fade is the segment's edge fade duration.
const Fade = 0.15

// Skippable allows activation input to end the brand segment.
const Skippable = true

// Background matches the compiled animation's #020B14 backdrop.
var Background = render.RGBA8(2, 11, 20, 255)

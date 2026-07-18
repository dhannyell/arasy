// Package particle defines the data-driven particle-effect vocabulary and
// the canonical CPU simulation that software backends run and the GPU
// backend mirrors. Particle content is render-only: it never enters
// snapshots, checksums, replays or recordings.
package particle

import "github.com/dhannyell/frameforge/math2d"

// MaxCapacity caps one emitter so its WebGPU particle storage stays at about
// 3.5 MiB (65,536 particles x 56 bytes across the hot/cold buffers). CPU
// fallbacks use the same limit to preserve backend parity and keep pool
// initialization bounded.
const MaxCapacity = 65536

// Blend selects how a particle quad is composited.
type Blend uint8

const (
	BlendAlpha    Blend = iota // premultiplied source-over
	BlendAdditive              // premultiplied additive (fire, sparks, glow)
)

// Range is a [Min, Max] interval sampled uniformly per particle.
type Range struct{ Min, Max float64 }

type EffectDecl struct {
	Name     string
	Texture  string // manifest texture name; "" draws untextured quads
	Blend    Blend
	Capacity int // particle pool size, clamped to MaxCapacity

	EmitRate   float64      // particles/second at Intensity 1
	BurstSize  int          // particles emitted per burst trigger
	Lifetime   Range        // seconds
	Speed      Range        // world units/second
	Direction  float64      // base emission direction, radians
	Spread     float64      // half-angle around Direction, radians
	Shape      EmitShape    // spawn region around the emitter position
	EmitRadius float64      // disc/ring radius, world units
	EmitExtent math2d.Vec2  // rect half-extents, world units (axis-aligned)
	VelMode    VelocityMode // initial velocity direction rule
	Gravity    math2d.Vec2  // world units/s^2
	Drag       float64      // fraction of velocity kept per second (1 = none)
	SizeStart  Range        // world units (quad edge)
	SizeEnd    Range
	ColorStart [4]float64 // straight-alpha RGBA, 0..1
	ColorEnd   [4]float64
	Spin       float64 // max |angular velocity|, radians/second
	Rotation   Range   // initial rotation, radians
	WobbleAmp  float64 // lateral oscillation amplitude, world units
	WobbleFreq float64 // oscillations per second
	FadeIn     float64 // fraction of life ramping alpha 0..1 (0 = none)
	FlipXFreq  float64 // fake 3D flips per second by scaling local X through zero
}

// EmitShape selects where new particles spawn around the emitter position.
type EmitShape uint8

const (
	EmitPoint EmitShape = iota // exactly at the emitter position
	EmitDisc                   // uniform over a disc of radius EmitRadius
	EmitRect                   // uniform over an axis-aligned rect (EmitExtent)
	EmitRing                   // on the circle of radius EmitRadius
)

// VelocityMode selects how a particle's initial velocity direction is
// chosen. Radial and tangential derive it from the spawn offset, so they
// require a non-point Shape (manifest validation enforces this).
type VelocityMode uint8

const (
	VelCone       VelocityMode = iota // Direction + command Dir ± Spread
	VelRadial                         // away from the emitter origin
	VelTangential                     // perpendicular to the spawn offset
)

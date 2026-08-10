package render

import "github.com/dhannyell/arasy/math2d"

// PostEffect is one parameterized effect in a PostChain. The closed interface
// keeps the render vocabulary finite so every backend can implement the same
// canonical semantics.
type PostEffect interface {
	isPostEffect()
}

// Vignette darkens or tints pixels toward the viewport edges.
type Vignette struct {
	// Color is the color the edge approaches.
	Color Color
	// Strength controls the maximum mix with Color. Zero is identity.
	Strength float64
	// Softness controls how far the edge falloff reaches. Zero uses a hard
	// edge; renderers clamp the value to their supported range.
	Softness float64
}

// ColorGrade adjusts brightness, contrast, saturation, and a multiplicative
// tint for the complete post-processed image.
type ColorGrade struct {
	// Brightness is added after contrast. Zero is neutral.
	Brightness float64
	// Contrast is an offset from one: zero is neutral, and one doubles the
	// contrast.
	Contrast float64
	// Saturation is an offset from one: zero is neutral, and -1 produces
	// grayscale.
	Saturation float64
	// Tint multiplies RGB. A zero tint is treated as white, so the zero value
	// remains identity.
	Tint Color
}

// Pixelate samples the image in square blocks of Size pixels.
type Pixelate struct {
	// Size is the block width and height in pixels. Zero is identity.
	Size float64
}

// Bloom extracts bright pixels, blurs them, and adds the result to the image.
type Bloom struct {
	// Threshold is the brightness above which pixels contribute to bloom.
	Threshold float64
	// Intensity controls the additive bloom contribution. Zero is identity.
	Intensity float64
	// Passes controls the dual-Kawase pyramid. Zero uses the default of three;
	// active values are clamped to the range [1, 5].
	Passes int
}

// ChromaticAberration offsets red and blue samples in opposite radial
// directions.
type ChromaticAberration struct {
	// Strength controls the radial offset. Zero is identity.
	Strength float64
}

// Shockwave refracts pixels around a moving ring whose geometry is authored
// in world units.
type Shockwave struct {
	// Center is the ring centre in world coordinates.
	Center math2d.Vec2
	// Radius is the ring radius in world units.
	Radius float64
	// Width is the ring width in world units. Zero is identity.
	Width float64
	// Strength controls the refraction offset. Zero is identity.
	Strength float64
}

// LensRain overlays deterministic camera droplets. Its output is a pure
// function of the source pixel, Time from PostChain, and Seed.
type LensRain struct {
	// Density controls the number of droplet cells. Zero is identity. An
	// The standard density is 0.4 when the caller wants the default active
	// effect; zero remains the explicit disabled value.
	Density float64
	// Size is the nominal droplet size in pixels. Zero uses 48.
	Size float64
	// Speed controls the falling rate. Zero uses 0.15.
	Speed float64
	// Refraction selects the N2 refractive variant. Zero keeps the N1 overlay.
	Refraction float64
	// Seed selects the deterministic droplet layout.
	Seed uint64
}

// PostChain applies a user-authored chain of fullscreen effects to the frame.
// It is render-only: it travels as a Command and is not simulation state, so
// it does not enter snapshots, checksums, recordings, or replay. The last
// PostChain encountered in a frame wins. BelowLayer zero covers the full
// frame; otherwise only commands with Layer() < BelowLayer are processed and
// the remaining commands are drawn over the result, which keeps a HUD clean.
// The recommended order is distortion effects (Shockwave and LensRain), then
// Bloom, ColorGrade, and Vignette; the chain remains user-defined and this
// order is not enforced.
type PostChain struct {
	BaseCommand
	// Effects are applied in slice order.
	Effects []PostEffect
	// BelowLayer separates the processed scene from commands drawn above it.
	// Zero means the whole frame.
	BelowLayer int
	// Time is the render clock used by time-dependent effects such as
	// LensRain.
	Time float64
}

func (Vignette) isPostEffect()            {}
func (ColorGrade) isPostEffect()          {}
func (Pixelate) isPostEffect()            {}
func (Bloom) isPostEffect()               {}
func (ChromaticAberration) isPostEffect() {}
func (Shockwave) isPostEffect()           {}
func (LensRain) isPostEffect()            {}

func (PostChain) isRenderCommand() {}

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

// LensRain overlays deterministic camera droplets: two static layers of
// condensation plus larger splash droplets that appear, hold and gradually
// erode away, landing at a new position every lifecycle. Its output is a
// pure function of the source pixel, Time from PostChain, and Seed.
type LensRain struct {
	// Density controls the number of droplet cells. Zero is identity.
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

// Blur softens the image with a fullscreen Kawase blur pyramid.
type Blur struct {
	// Strength is the mix between the sharp and blurred image, clamped to
	// [0, 1]. Zero is identity.
	Strength float64
	// Passes controls the dual-Kawase pyramid. Zero uses the default of three;
	// active values are clamped to the range [1, 5].
	Passes int
}

// Fade mixes the whole image toward a solid Color, for screen transitions.
type Fade struct {
	// Color is the color the image fades toward.
	Color Color
	// Amount is the mix with Color, clamped to [0, 1]. Zero is identity.
	Amount float64
}

// CircleWipe reveals or covers the screen with a circle centered in world
// coordinates, for iris-style transitions.
type CircleWipe struct {
	// Center is the circle center in world coordinates.
	Center math2d.Vec2
	// Color fills the covered area.
	Color Color
	// Progress goes from 0 (fully visible) to 1 (fully covered by Color).
	// Values at or below zero are identity.
	Progress float64
	// Softness feathers the circle edge, clamped to [0, 1].
	Softness float64
}

// Dissolve covers the screen with Color one cell at a time, in a
// deterministic pattern keyed by Seed.
type Dissolve struct {
	// Color fills a covered cell.
	Color Color
	// Amount is the covered fraction of cells, clamped to [0, 1]. Values at or
	// below zero are identity.
	Amount float64
	// CellSize is the cell width and height in pixels. Zero or less uses 8.
	CellSize float64
	// Seed selects the deterministic cell pattern.
	Seed uint64
}

// FilmGrain adds animated per-pixel luminance noise.
type FilmGrain struct {
	// Intensity controls the noise amplitude. Zero is identity.
	Intensity float64
	// Size is the grain width and height in pixels. Zero or less uses 1.
	Size float64
	// Seed selects the deterministic noise pattern.
	Seed uint64
}

// Scanlines darkens periodic horizontal lines, for a CRT monitor look.
type Scanlines struct {
	// Intensity is how dark each line gets, clamped to [0, 1]. Zero is
	// identity.
	Intensity float64
	// Thickness is the line period in pixels. Zero or less uses 3.
	Thickness float64
	// MaskStrength dims the two off-triad channels of each pixel column,
	// like a CRT aperture grille, clamped to [0, 1]. Zero disables the mask.
	MaskStrength float64
}

// RadialBlur streaks the image toward a focus point, for speed or warp
// emphasis.
type RadialBlur struct {
	// Center is the blur focus in world coordinates.
	Center math2d.Vec2
	// Strength is the streak length factor, clamped to [0, 1]. Zero is
	// identity.
	Strength float64
}

// FocusBlur blurs the frame everywhere but around its focus points, the
// depth-of-field of a lens whose plane of focus follows the action. Points
// are world positions; the nearest one decides each pixel's sharpness.
type FocusBlur struct {
	// Points are kept sharp. At most 16 are used; none blurs the whole frame.
	Points []math2d.Vec2
	// Extent is the half-size of the sharp region around each point and
	// Rounding its corner radius, both in world units. Rounding is clamped
	// to the smaller half-extent: equal to it the region is an ellipse-like
	// stadium, zero a rectangle. A zero Extent uses Rounding as a circle.
	Extent   math2d.Vec2
	Rounding float64
	// Falloff is how much farther past the region's edge the blur takes to
	// reach Strength, in world units.
	Falloff float64
	// Strength is the mix between the sharp and blurred image at full
	// distance, clamped to [0, 1]. Zero is identity.
	Strength float64
	// Passes controls the dual-Kawase pyramid. Zero uses the default of three;
	// active values are clamped to the range [1, 5].
	Passes int
}

// FocusBlurMaxPoints is how many focus points a FocusBlur carries to the
// backends; extras are ignored.
const FocusBlurMaxPoints = 16

// HalftoneShape selects how the dot grows as the tone darkens.
type HalftoneShape int

const (
	// HalftoneRound grows a disc from the cell centre.
	HalftoneRound HalftoneShape = iota
	// HalftoneCross grows an X along the cell diagonals.
	HalftoneCross
	// HalftonePlus grows a + along the axes from two centres per cell, on
	// a checkerboard lattice: at mid tones the arms of neighbouring dots
	// nearly meet and leave single-pixel diamonds of the other tone
	// between them, the screen of classic monochrome dithered art. Best
	// with an even CellSize; 4 gives a five-pixel + per dot.
	HalftonePlus
)

// Halftone quantises luma to Levels tones and dithers the remainder with a
// screen-locked dot grid, the cluster-dot screen of print and of monochrome
// dithered games.
type Halftone struct {
	// CellSize is the dot grid pitch in dither pixels, rounded to whole
	// pixels and capped at 16. Zero or fewer uses 8.
	CellSize float64
	// Pixel is the size of one dither pixel in screen pixels, so the dots
	// can share the grid of a scene drawn at lower resolution (a Pixelate
	// of the same size before it). Zero or fewer uses 1.
	Pixel float64
	// Levels is how many tones survive. Zero or fewer uses 2.
	Levels int
	// Shape selects how the dot grows as the tone darkens.
	Shape HalftoneShape
	// Strength mixes source and result, clamped to [0,1]. Zero is identity.
	Strength float64
	// Noise is the amplitude, in threshold units, of a per-pixel random
	// offset added to the dither threshold, so pixels near a tone boundary
	// flicker between tones while flat regions hold still. Zero is a static
	// screen; around 0.2 reads as film grain.
	Noise float64
	// NoiseRate is how many times per second the noise is redrawn. Zero or
	// fewer uses 12; the reference material runs well below the frame rate.
	NoiseRate float64
	// Seed keys the noise.
	Seed uint32
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
func (Blur) isPostEffect()                {}
func (Fade) isPostEffect()                {}
func (CircleWipe) isPostEffect()          {}
func (Dissolve) isPostEffect()            {}
func (FilmGrain) isPostEffect()           {}
func (Scanlines) isPostEffect()           {}
func (RadialBlur) isPostEffect()          {}
func (Halftone) isPostEffect()            {}
func (FocusBlur) isPostEffect()           {}

func (PostChain) isRenderCommand() {}

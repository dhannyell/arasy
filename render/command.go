package render

import "github.com/dhannyell/arasy/math2d"

type TextAlign uint8
type TextBaseline uint8

// FontID identifies a font declared in the game's asset manifest. Zero
// means the built-in engine font. Like TextureID, values are stable only
// for the current run — never persist a FontID; resolve by name through
// the manifest.
type FontID uint32

const (
	AlignLeft TextAlign = iota
	AlignCenter
	AlignRight
)

const (
	BaselineAlphabetic TextBaseline = iota
	BaselineTop
	BaselineMiddle
	BaselineBottom
)

// Command represents a render operation.
type Command interface {
	isRenderCommand()
	Layer() int
}

// TextureID identifies a texture declared in the game's asset manifest.
// IDs are assigned deterministically for a given asset set, but only hold
// for the current run - never persist a TextureID; always resolve it by
// name through the manifest.
type TextureID uint32

type BaseCommand struct {
	LayerValue int

	// Clip, when non-zero, restricts this command to the rectangle in world
	// coordinates. Zero value = no clipping.
	Clip math2d.Rect

	// Transform, when non-identity (and non-zero), maps this command's
	// world-space geometry through the affine matrix before the camera is
	// applied. Zero value = identity = no transform. Pivoted transforms:
	// build with Affine.About(pivot). Decorative use only for now —
	// interactive widget hit-testing under transform is deferred.
	Transform math2d.Affine
}

func (b BaseCommand) Layer() int {
	return b.LayerValue
}

// DrawRect describes a filled rectangle draw command.
type DrawRect struct {
	BaseCommand
	Rect         math2d.Rect
	Fill         Color
	Stroke       Color
	StrokeWidth  float64
	CornerRadius float64
}

// DrawCircle describes a filled circle draw command.
type DrawCircle struct {
	BaseCommand
	Center      math2d.Vec2
	Radius      float64
	Fill        Color
	Stroke      Color
	StrokeWidth float64
}

// DrawEllipse describes an axis-aligned ellipse draw command in world coodinates.
type DrawEllipse struct {
	BaseCommand
	Center      math2d.Vec2
	RadiusX     float64
	RadiusY     float64
	Fill        Color
	Stroke      Color
	StrokeWidth float64
}

// DrawPoint describes a filled point marker draw command.
type DrawPoint struct {
	BaseCommand
	Center math2d.Vec2
	Radius float64
	Color  Color
}

// DrawLine describes a line draw command.
type DrawLine struct {
	BaseCommand
	From  math2d.Vec2
	To    math2d.Vec2
	Width float64
	Color Color
}

// DrawText describes a text draw command.
type DrawText struct {
	BaseCommand
	Position math2d.Vec2
	Text     string
	Color    Color
	Size     float64
	Font     FontID
	// Metrics optionally carries a prepared canonical measurement for static
	// text. Zero value asks the backend to measure Text normally.
	Metrics TextMetrics
	// LetterSpacing adds world-space distance between adjacent glyphs.
	LetterSpacing float64
	Align         TextAlign
	Baseline      TextBaseline

	// Rotation is applied around Position, in radians. Zero keeps text
	// unrotated.
	//
	// When both Rotation and BaseCommand.Transform are set, Rotation applies
	// first (around Position), then Transform maps the result. Prefer
	// Transform in new code.
	Rotation float64
}

// DrawArc describes an arc stroke draw command.
type DrawArc struct {
	BaseCommand
	Center     math2d.Vec2
	Radius     float64
	StartAngle float64
	EndAngle   float64
	Width      float64
	Color      Color
}

// DrawPolygon describes a polygon draw command
type DrawPolygon struct {
	BaseCommand
	Points      []math2d.Vec2
	Fill        Color
	Stroke      Color
	StrokeWidth float64
}

// DrawTriangles draws indexed solid-color triangles. Indices are interpreted
// in groups of three and must reference Vertices.
type DrawTriangles struct {
	BaseCommand
	Vertices []math2d.Vec2
	Indices  []uint32
	Color    Color
}

// DrawImage draws a texture, or a sub-rectangle of it, into a destination
// rectangle in world coordinates.
type DrawImage struct {
	BaseCommand
	Texture TextureID
	// Dest is the destination rectangle in world coordinates.
	Dest math2d.Rect
	// Src selects a sub-rectangle of the texture in pixel coordinates.
	// Zero value = the whole texture (sprite sheets pass a sprite rect).
	Src math2d.Rect
	// Tint multiplies the texture color channel-wise (including alpha).
	// Zero value = no tint (drawn as-is); opaque white behaves the same.
	Tint Color
	// Rotation is applied around the center of Dest, in radians. Zero
	// keeps the image axis-aligned.
	//
	// When both Rotation and BaseCommand.Transform are set, Rotation applies
	// first (around the center of Dest), then Transform maps the result.
	// Prefer Transform in new code.
	Rotation float64
}

// DrawImageQuad draws a texture, or a sub-rectangle of it, mapped onto an
// arbitrary convex quadrilateral in world coordinates with perspective-
// correct interpolation (projective mapping). Corners are given in the
// order top-left, top-right, bottom-right, bottom-left of the SOURCE
// image; a plain rectangle reproduces DrawImage. Non-convex or degenerate
// corner sets are skipped by backends (a one-time warning is logged).
type DrawImageQuad struct {
	BaseCommand
	Texture TextureID
	// Corners in world coordinates: source top-left, top-right,
	// bottom-right, bottom-left.
	Corners [4]math2d.Vec2
	// Src selects a sub-rectangle of the texture in pixel coordinates.
	// Zero value = the whole texture.
	Src math2d.Rect
	// Tint multiplies the texture color channel-wise (including alpha).
	// Zero value = no tint.
	Tint Color
}

// ParticleEffectID identifies a particle effect declared in the asset
// manifest. Like TextureID, values are stable only for the current run.
type ParticleEffectID uint32

// EmitterID identifies one live instance of an effect.
// IDs are chosen by the game, like layers; backends key their
// per-instance state on (Effect, Emitter).
type EmitterID uint32

// DrawParticles drives one particle-emitter instance for this frame.
// Emitting the command each frame keeps the emitter alive and feeds it
// time; backends that simulate on the GPU never report particle state
// back. Content is render-only: never captured by snapshots, checksums,
// replays or recordings. An emitter not commanded for a few seconds is
// released by backends. If (Effect, Emitter) occurs more than once in a
// frame, the first command advances simulation; later commands with the
// same Seed only draw that state again, while conflicting seeds are skipped.
type DrawParticles struct {
	BaseCommand
	Effect  ParticleEffectID
	Emitter EmitterID
	// Pos is the emitter origin in world coordinates.
	Pos math2d.Vec2
	// EmitScale scales the effect's declared spawn shape per axis. Zero
	// components default to 1 for backward compatibility.
	EmitScale math2d.Vec2
	// Dir rotates the effect's declared emission direction, radians.
	Dir float64
	// Intensity scales the declared emit rate; 0 stops continuous
	// emission (live particles still age out).
	Intensity float64
	// Burst is a cumulative trigger counter: each increment emits the
	// effect's BurstSize particles once. Keep it monotonic.
	Burst uint32
	// Time is the render clock (RenderContext.Time).
	Time float64
	// Dt is the render frame delta (RenderContext.Dt).
	Dt float64
	// Seed differentiates instances; same seed + same command stream =
	// same spawn sequence on every backend.
	Seed uint64
}

func (DrawRect) isRenderCommand()        {}
func (DrawCircle) isRenderCommand()      {}
func (DrawEllipse) isRenderCommand()     {}
func (DrawPoint) isRenderCommand()       {}
func (DrawLine) isRenderCommand()        {}
func (DrawText) isRenderCommand()        {}
func (DrawArc) isRenderCommand()         {}
func (DrawPolygon) isRenderCommand()     {}
func (DrawTriangles) isRenderCommand()   {}
func (DrawImage) isRenderCommand()       {}
func (DrawImageQuad) isRenderCommand()   {}
func (DrawParticles) isRenderCommand()   {}
func (DrawSpriteBatch) isRenderCommand() {}
func (DrawTextBatch) isRenderCommand()   {}

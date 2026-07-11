package render

import "github.com/dhannyell/frameforge/math2d"

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
	FontID   FontID
	Align    TextAlign
	Baseline TextBaseline

	// Rotation is applied around Position, in radians. Zero keeps text
	// unrotated.
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

func (DrawRect) isRenderCommand()      {}
func (DrawCircle) isRenderCommand()    {}
func (DrawPoint) isRenderCommand()     {}
func (DrawLine) isRenderCommand()      {}
func (DrawText) isRenderCommand()      {}
func (DrawArc) isRenderCommand()       {}
func (DrawPolygon) isRenderCommand()   {}
func (DrawImage) isRenderCommand()     {}
func (DrawImageQuad) isRenderCommand() {}

package render

import "github.com/dhannyell/arasy/math2d"

// SpriteBatchID identifies a retained sprite batch published by the game.
// IDs are chosen by the game, are valid only for the current run, and are
// independent from the TextBatchID namespace — never persist a
// SpriteBatchID.
type SpriteBatchID uint32

// TextBatchID identifies a retained text batch published by the game. IDs
// are chosen by the game, are valid only for the current run, and are
// independent from the SpriteBatchID namespace — never persist a
// TextBatchID.
type TextBatchID uint32

// SpriteFlip mirrors the sampled sprite content inside its destination.
type SpriteFlip uint8

const (
	// FlipNone keeps the sampled sprite orientation unchanged.
	FlipNone SpriteFlip = 0
	// FlipX mirrors the sampled sprite horizontally.
	FlipX SpriteFlip = 1 << 0
	// FlipY mirrors the sampled sprite vertically.
	FlipY SpriteFlip = 1 << 1
)

// SpriteInstance is one retained DrawImage-style quad. Coordinates and Src
// use the same world/pixel conventions as DrawImage. Content is render-only:
// it never enters checksums, snapshots, replays or recordings.
type SpriteInstance struct {
	Texture TextureID
	// Dest is the destination rectangle in world coordinates.
	Dest math2d.Rect
	// Src is the base flip-book cell, or the sub-rectangle to draw for a
	// static sprite, in texture pixel coordinates. Zero value = the whole
	// texture.
	Src math2d.Rect
	// Tint multiplies the texture color channel-wise (including alpha).
	// Zero value = no tint (drawn as-is), same as DrawImage.
	Tint Color
	// Rotation is applied around the center of Dest, in radians. Zero
	// keeps the sprite axis-aligned.
	Rotation float64
	// Flip mirrors the sampled Src content inside Dest before Rotation.
	Flip SpriteFlip
	// ProjectCorners makes the backend transform all four Dest corners through
	// the camera. Use it for ground content, such as tilemaps, whose shape must
	// follow an isometric or tilted world view. The zero value keeps the sprite
	// corners screen-space billboards.
	ProjectCorners bool
	// Animation is optional presentation data; the zero value is inert.
	Animation SpriteAnimation
}

// FlipQuadCorners returns the corner order that mirrors source sampling for a
// quad. DrawImageQuad has no flip field, so projected sprite batches use this
// when they lower a SpriteInstance to a quad command.
func FlipQuadCorners(corners [4]math2d.Vec2, flip SpriteFlip) [4]math2d.Vec2 {
	if flip&FlipX != 0 {
		corners[0], corners[1] = corners[1], corners[0]
		corners[3], corners[2] = corners[2], corners[3]
	}
	if flip&FlipY != 0 {
		corners[0], corners[3] = corners[3], corners[0]
		corners[1], corners[2] = corners[2], corners[1]
	}
	return corners
}

// SpriteAnimation is optional per-instance presentation data for a retained
// sprite. Frames <= 1 is static. Src is the base cell; frames advance
// row-major through equal cells in one texture. Bounce is the vertical
// bounce amplitude in world units, scaled by the batch animation uniforms.
type SpriteAnimation struct {
	Frames uint16
	Rate   float32
	Phase  float32
	Bounce float32
}

// SpritePulse describes the single radial pulse of a sprite batch: sprites
// whose Dest center sits near Radius world units from Center get their
// bounce boosted by a gaussian of the given Width. Width <= 0 or Boost == 0
// disables the pulse.
type SpritePulse struct {
	Center math2d.Vec2
	Radius float64
	Width  float64
	Boost  float64
}

// SpriteBatchAnimation carries the per-draw animation uniforms of a
// DrawSpriteBatch command. Time is the render clock in seconds, wrapped
// before evaluation so CPU and GPU agree. BounceHz <= 0 or Intensity == 0
// disables the vertical bounce for the whole batch.
type SpriteBatchAnimation struct {
	Time      float64
	BounceHz  float64
	Intensity float64
	Pulse     SpritePulse
}

// DrawSpriteBatch draws one retained sprite batch for this frame. Batch
// content lives in the backend; the command carries only the batch ID and
// the per-draw animation uniforms.
//
// The command is a painter-order boundary: instances draw in slice order at
// the command's position in the frame. One batch owns exactly one layer and
// one clip; content needing another layer or clip uses another batch. A
// command referencing a missing or deleted batch is skipped by backends
// with a one-time warning. Batch content is render-only: it never enters
// checksums, snapshots, replays or recordings.
type DrawSpriteBatch struct {
	BaseCommand
	Batch     SpriteBatchID
	Animation SpriteBatchAnimation
}

// TextLabel mirrors DrawText without BaseCommand: DrawTextBatch owns one
// layer and clip, so content requiring another layer or clip uses another
// batch. Content is render-only: it never enters checksums, snapshots,
// replays or recordings.
type TextLabel struct {
	Position math2d.Vec2
	Text     string
	Color    Color
	Size     float64
	Font     FontID
	Align    TextAlign
	Baseline TextBaseline

	// Rotation is applied around Position, in radians. Zero keeps text
	// unrotated.
	Rotation float64
}

// DrawTextBatch draws one retained text batch for this frame. Batch content
// lives in the backend; the command carries only the batch ID.
//
// The command is a painter-order boundary: labels draw in slice order at
// the command's position in the frame. One batch owns exactly one layer and
// one clip; content needing another layer or clip uses another batch. A
// command referencing a missing or deleted batch is skipped by backends
// with a one-time warning. Batch content is render-only: it never enters
// checksums, snapshots, replays or recordings.
type DrawTextBatch struct {
	BaseCommand
	Batch TextBatchID
}

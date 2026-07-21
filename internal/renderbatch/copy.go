package renderbatch

import "github.com/dhannyell/frameforge/render"

// CopySprites returns a defensive copy of instances for a backend to store.
// The caller may reuse or mutate its slice after the call without affecting
// the copy. An empty input yields nil, which backends keep as an empty
// (still existing) batch.
func CopySprites(in []render.SpriteInstance) []render.SpriteInstance {
	if len(in) == 0 {
		return nil
	}
	out := make([]render.SpriteInstance, len(in))
	copy(out, in)
	return out
}

// CopyLabels returns a defensive copy of labels for a backend to store.
// The caller may reuse or mutate its slice after the call without affecting
// the copy. An empty input yields nil, which backends keep as an empty
// (still existing) batch.
func CopyLabels(in []render.TextLabel) []render.TextLabel {
	if len(in) == 0 {
		return nil
	}
	out := make([]render.TextLabel, len(in))
	copy(out, in)
	return out
}

package wasm

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// textBatchProjKey identifies every input that changes a retained text
// batch's packed screen-space records: the camera's resolved world/viewport
// mapping (which determines WorldToScreen and pixel size) and the canvas
// size. Comparing this canonical, fully-resolved value instead of the raw
// Camera avoids false invalidation from float noise in unrelated Camera
// fields, and false negatives from AspectFit/AspectExpand producing the same
// resolved mapping for different raw inputs.
type textBatchProjKey struct {
	resolved render.ResolvedCamera
	w, h     int
}

func currentTextBatchProjKey(cam render.Camera, w, h int) textBatchProjKey {
	return textBatchProjKey{resolved: cam.Resolve(), w: w, h: h}
}

// textBatchScreenView returns the screen-space rect a retained text batch
// must intersect to produce pixels: the canvas rect, tightened by the
// command's world-space clip projected to screen when set. ok is false when
// the canvas has no usable area, in which case the caller draws without
// culling.
func textBatchScreenView(cam render.Camera, w, h int, clip math2d.Rect) (math2d.Rect, bool) {
	if w <= 0 || h <= 0 {
		return math2d.Rect{}, false
	}
	view := math2d.Rect{Max: math2d.Vec2{X: float64(w), Y: float64(h)}}
	if clip == (math2d.Rect{}) {
		return view, true
	}
	a := cam.WorldToScreen(clip.Min)
	b := cam.WorldToScreen(clip.Max)
	view.Min.X = max(view.Min.X, math.Min(a.X, b.X))
	view.Min.Y = max(view.Min.Y, math.Min(a.Y, b.Y))
	view.Max.X = min(view.Max.X, math.Max(a.X, b.X))
	view.Max.Y = min(view.Max.Y, math.Max(a.Y, b.Y))
	return view, true
}

// collectTextBatchIDs returns the distinct DrawTextBatch IDs referenced in
// commands, in first-appearance order. Order does not affect painter order
// (tess.Build owns that separately); this only drives which retained batches
// need atlas-coverage preparation and repack resolution this frame.
func collectTextBatchIDs(commands []render.Command) []render.TextBatchID {
	return collectTextBatchIDsInto(nil, commands)
}

func collectTextBatchIDsInto(ids []render.TextBatchID, commands []render.Command) []render.TextBatchID {
	ids = ids[:0]
	for _, command := range commands {
		cmd, ok := command.(render.DrawTextBatch)
		if !ok {
			continue
		}
		duplicate := false
		for _, id := range ids {
			if id == cmd.Batch {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		ids = append(ids, cmd.Batch)
	}
	return ids
}

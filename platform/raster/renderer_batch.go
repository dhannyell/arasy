package raster

import (
	"image"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// ReplaceSpriteBatch stores the whole content of a retained sprite batch,
// taking ownership of the slice. An empty slice keeps an empty batch; only
// DeleteSpriteBatch removes the resource. Batch content is render-only.
func (r *Renderer) ReplaceSpriteBatch(id render.SpriteBatchID, instances []render.SpriteInstance) {
	if r.spriteBatches == nil {
		r.spriteBatches = make(map[render.SpriteBatchID][]render.SpriteInstance)
	}
	r.spriteBatches[id] = instances
}

// DeleteSpriteBatch removes a retained sprite batch.
func (r *Renderer) DeleteSpriteBatch(id render.SpriteBatchID) {
	delete(r.spriteBatches, id)
}

// ReplaceTextBatch stores the whole content of a retained text batch, taking
// ownership of the slice. An empty slice keeps an empty batch; only
// DeleteTextBatch removes the resource. Batch content is render-only.
func (r *Renderer) ReplaceTextBatch(id render.TextBatchID, labels []render.TextLabel) {
	if r.textBatches == nil {
		r.textBatches = make(map[render.TextBatchID][]render.TextLabel)
	}
	r.textBatches[id] = labels
}

// DeleteTextBatch removes a retained text batch.
func (r *Renderer) DeleteTextBatch(id render.TextBatchID) {
	delete(r.textBatches, id)
}

// drawSpriteBatch draws one retained sprite batch at the command's painter
// position. The whole batch is culled when its animation-inflated bounds do
// not intersect the rendered view (composed with the command clip); visible
// instances convert to the same drawImage path as streaming DrawImage, in
// slice order, so retained and streamed sprites are pixel-equivalent.
func (r *Renderer) drawSpriteBatch(img *image.RGBA, command render.DrawSpriteBatch) {
	instances, ok := r.spriteBatches[command.Batch]
	if !ok {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "sprite-batch-missing", ID: uint64(command.Batch)},
			"raster: DrawSpriteBatch references a missing or deleted batch; command skipped")
		return
	}
	if len(instances) == 0 {
		return
	}

	if view, ok := r.batchView(command.Clip); ok {
		bounds, boundsOK := renderbatch.InflatedBatchBoundsForCamera(instances, command.Animation, r.Camera)
		if !boundsOK {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "sprite-batch-non-finite", ID: uint64(command.Batch)},
				"raster: sprite batch has non-finite geometry; command skipped")
			return
		}
		if !renderbatch.IntersectsView(bounds, view) {
			return
		}
	}

	for i := range instances {
		inst := instances[i]
		if !renderbatch.IsFiniteSpriteGeometry(inst) {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "sprite-instance-non-finite", ID: uint64(command.Batch)},
				"raster: sprite instance has non-finite geometry; instance skipped")
			continue
		}

		src := inst.Src
		if tex := r.Textures.Lookup(inst.Texture); tex != nil {
			b := tex.Bounds()
			src = renderbatch.AnimatedSrc(inst, command.Animation.Time, b.Dx(), b.Dy())
		}

		dest := inst.Dest
		if offsetY := renderbatch.BounceOffsetY(inst, command.Animation); offsetY != 0 {
			dest.Min.Y += offsetY
			dest.Max.Y += offsetY
		}

		r.drawImage(img, render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: command.LayerValue, Clip: command.Clip},
			Texture:     inst.Texture,
			Dest:        dest,
			Src:         src,
			Tint:        inst.Tint,
			Rotation:    inst.Rotation,
		})
	}
}

// drawTextBatch draws one retained text batch at the command's painter
// position: each label converts to the same drawText path as streaming
// DrawText, in slice order, sharing the batch's layer and clip.
func (r *Renderer) drawTextBatch(img *image.RGBA, command render.DrawTextBatch) {
	labels, ok := r.textBatches[command.Batch]
	if !ok {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "text-batch-missing", ID: uint64(command.Batch)},
			"raster: DrawTextBatch references a missing or deleted batch; command skipped")
		return
	}

	for i := range labels {
		label := labels[i]
		r.drawText(img, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: command.LayerValue, Clip: command.Clip},
			Position:    label.Position,
			Text:        label.Text,
			Color:       label.Color,
			Size:        label.Size,
			Font:        label.Font,
			Align:       label.Align,
			Baseline:    label.Baseline,
			Rotation:    label.Rotation,
		})
	}
}

// batchView returns the world-space rect a batch must intersect to produce
// pixels: the camera's resolved visible world, tightened by the command clip
// when set. ok is false when the camera has no usable world area, in which
// case the caller draws without culling.
func (r *Renderer) batchView(clip math2d.Rect) (math2d.Rect, bool) {
	view := r.Camera.Resolve().World
	if view.Width() <= 0 || view.Height() <= 0 {
		return math2d.Rect{}, false
	}
	if clip != (math2d.Rect{}) {
		view.Min.X = max(view.Min.X, clip.Min.X)
		view.Min.Y = max(view.Min.Y, clip.Min.Y)
		view.Max.X = min(view.Max.X, clip.Max.X)
		view.Max.Y = min(view.Max.Y, clip.Max.Y)
		// An empty intersection still culls: IntersectsView requires a
		// positive-area overlap, and a degenerate rect overlaps nothing.
	}
	return view, true
}

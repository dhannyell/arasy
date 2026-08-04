//go:build js && wasm

package wasm

import (
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// canvasBatchView returns the world-space rect a batch must intersect to
// produce pixels: the camera's resolved visible world, tightened by the
// command clip when set. ok is false when the camera has no usable world
// area, in which case the caller draws without culling.
func canvasBatchView(camera render.Camera, clip math2d.Rect) (math2d.Rect, bool) {
	view := camera.Resolve().World
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

func (s canvasSink) VisitSpriteBatch(command render.DrawSpriteBatch) {
	instances, ok := s.canvas.batches.sprites[command.Batch]
	if !ok {
		s.canvas.diagnostics.Log(s.canvas.log, slog.LevelWarn,
			diagnostic.Key{Code: "canvas-sprite-batch-missing", ID: uint64(command.Batch)},
			"Canvas2D: DrawSpriteBatch references a missing or deleted batch; command skipped")
		return
	}
	if len(instances) == 0 {
		return
	}
	s.withClipAndTransform(command.Clip, math2d.Affine{}, render.BlendAlpha, func() {
		s.canvas.drawSpriteBatch(s.camera, command, instances)
	})
}

// drawSpriteBatch mirrors the raster reference: whole-batch culling against
// the animation-inflated bounds, then one drawImage per instance in slice
// order through the same code path as streaming DrawImage.
func (c *Canvas2D) drawSpriteBatch(camera render.Camera, command render.DrawSpriteBatch, instances []render.SpriteInstance) {
	if view, ok := canvasBatchView(camera, command.Clip); ok {
		bounds, boundsOK := renderbatch.InflatedBatchBoundsForCamera(instances, command.Animation, camera)
		if !boundsOK {
			c.diagnostics.Log(c.log, slog.LevelWarn,
				diagnostic.Key{Code: "canvas-sprite-batch-non-finite", ID: uint64(command.Batch)},
				"Canvas2D: sprite batch has non-finite geometry; command skipped")
			return
		}
		if !renderbatch.IntersectsView(bounds, view) {
			return
		}
	}

	for i := range instances {
		inst := instances[i]
		if !renderbatch.IsFiniteSpriteGeometry(inst) {
			c.diagnostics.Log(c.log, slog.LevelWarn,
				diagnostic.Key{Code: "canvas-sprite-instance-non-finite", ID: uint64(command.Batch)},
				"Canvas2D: sprite instance has non-finite geometry; instance skipped")
			continue
		}

		src := inst.Src
		if bitmap, ok := c.textures[inst.Texture]; ok && bitmap.Truthy() {
			src = renderbatch.AnimatedSrc(inst, command.Animation.Time,
				bitmap.Get("width").Int(), bitmap.Get("height").Int())
		}

		dest := inst.Dest
		if offsetY := renderbatch.BounceOffsetY(inst, command.Animation); offsetY != 0 {
			dest.Min.Y += offsetY
			dest.Max.Y += offsetY
		}

		// The batch clip is already applied by withClip around this loop.
		c.drawImage(render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: command.LayerValue},
			Texture:     inst.Texture,
			Dest:        dest,
			Src:         src,
			Tint:        inst.Tint,
			Rotation:    inst.Rotation,
		}, camera)
	}
}

func (s canvasSink) VisitTextBatch(command render.DrawTextBatch) {
	labels, ok := s.canvas.batches.text[command.Batch]
	if !ok {
		s.canvas.diagnostics.Log(s.canvas.log, slog.LevelWarn,
			diagnostic.Key{Code: "canvas-text-batch-missing", ID: uint64(command.Batch)},
			"Canvas2D: DrawTextBatch references a missing or deleted batch; command skipped")
		return
	}
	s.withClipAndTransform(command.Clip, math2d.Affine{}, render.BlendAlpha, func() {
		for i := range labels {
			label := labels[i]
			s.canvas.drawText(render.DrawText{
				BaseCommand: render.BaseCommand{LayerValue: command.LayerValue},
				Position:    label.Position,
				Text:        label.Text,
				Color:       label.Color,
				Size:        label.Size,
				Font:        label.Font,
				Align:       label.Align,
				Baseline:    label.Baseline,
				Rotation:    label.Rotation,
			}, s.camera)
		}
	})
}

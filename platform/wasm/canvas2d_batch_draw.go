//go:build js && wasm

package wasm

import (
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func (s canvasSink) VisitSpriteBatch(command render.DrawSpriteBatch) {
	s = s.forCommand(command)
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

// drawSpriteBatch renders a retained batch in stored order. It uses the same
// culling and image path as the raster backend.
func (c *Canvas2D) drawSpriteBatch(camera render.Camera, command render.DrawSpriteBatch, instances []render.SpriteInstance) {
	if view, ok := renderbatch.BatchView(camera, command.Clip); ok {
		bounds, boundsOK := renderbatch.InflatedBatchBoundsForCamera(instances, command.Animation, camera)
		if !boundsOK {
			c.diagnostics.Log(c.log, slog.LevelWarn,
				diagnostic.Key{Code: "canvas-sprite-batch-non-finite", ID: uint64(command.Batch)},
				"Canvas2D: sprite batch has non-finite geometry; command skipped")
			return
		}
		bounds = renderbatch.BoundsForView(bounds, camera.View)
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
		if inst.ProjectCorners {
			projected := inst
			projected.Dest = dest
			c.drawImageQuad(render.DrawImageQuad{
				BaseCommand: render.BaseCommand{LayerValue: command.LayerValue},
				Texture:     projected.Texture,
				Corners:     render.FlipQuadCorners(renderbatch.SpriteCorners(projected), projected.Flip),
				Src:         src,
				Tint:        projected.Tint,
			}, camera)
			continue
		}

		// The batch clip is already applied by withClip around this loop.
		c.drawImage(render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: command.LayerValue},
			Texture:     inst.Texture,
			Dest:        dest,
			Src:         src,
			Tint:        inst.Tint,
			Rotation:    inst.Rotation,
			Flip:        inst.Flip,
		}, camera)
	}
}

func (s canvasSink) VisitTextBatch(command render.DrawTextBatch) {
	s = s.forCommand(command)
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

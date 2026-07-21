package wasm

import (
	"log/slog"

	"github.com/dhannyell/frameforge/internal/diagnostic"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// Canvas2D retained-batch caps: the dev adapter truncates content beyond
// these limits with a one-time warning per batch. WebGPU and raster are the
// reference backends above the cap.
const (
	canvasSpriteBatchCap = 2000
	canvasTextBatchCap   = 500
)

// canvasBatchRegistry stores retained batch content for the Canvas2D dev
// adapter. Content is render-only: never snapshotted or recorded. Replace is
// wholesale; an empty slice keeps an empty batch and only delete removes the
// resource.
type canvasBatchRegistry struct {
	sprites     map[render.SpriteBatchID][]render.SpriteInstance
	text        map[render.TextBatchID][]render.TextLabel
	diagnostics diagnostic.Once
}

func (r *canvasBatchRegistry) applySprites(log *slog.Logger, updates []runtime.SpriteBatchUpdate) {
	for _, u := range updates {
		if u.Op == runtime.BatchDelete {
			delete(r.sprites, u.ID)
			continue
		}
		instances := u.Instances
		if len(instances) > canvasSpriteBatchCap {
			r.diagnostics.Log(log, slog.LevelWarn,
				diagnostic.Key{Code: "canvas-sprite-batch-cap", ID: uint64(u.ID)},
				"Canvas2D sprite batch exceeds the dev cap; truncating",
				"batch", u.ID, "cap", canvasSpriteBatchCap, "got", len(instances))
			instances = instances[:canvasSpriteBatchCap]
		}
		if r.sprites == nil {
			r.sprites = make(map[render.SpriteBatchID][]render.SpriteInstance)
		}
		r.sprites[u.ID] = instances
	}
}

func (r *canvasBatchRegistry) applyText(log *slog.Logger, updates []runtime.TextBatchUpdate) {
	for _, u := range updates {
		if u.Op == runtime.BatchDelete {
			delete(r.text, u.ID)
			continue
		}
		labels := u.Labels
		if len(labels) > canvasTextBatchCap {
			r.diagnostics.Log(log, slog.LevelWarn,
				diagnostic.Key{Code: "canvas-text-batch-cap", ID: uint64(u.ID)},
				"Canvas2D text batch exceeds the dev cap; truncating",
				"batch", u.ID, "cap", canvasTextBatchCap, "got", len(labels))
			labels = labels[:canvasTextBatchCap]
		}
		if r.text == nil {
			r.text = make(map[render.TextBatchID][]render.TextLabel)
		}
		r.text[u.ID] = labels
	}
}

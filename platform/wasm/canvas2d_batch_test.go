package wasm

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

func spriteUpdates(id render.SpriteBatchID, op runtime.BatchOp, n int) []runtime.SpriteBatchUpdate {
	var instances []render.SpriteInstance
	for i := 0; i < n; i++ {
		instances = append(instances, render.SpriteInstance{Texture: 1})
	}
	return []runtime.SpriteBatchUpdate{{ID: id, Op: op, Instances: instances}}
}

func textUpdates(id render.TextBatchID, op runtime.BatchOp, n int) []runtime.TextBatchUpdate {
	var labels []render.TextLabel
	for i := 0; i < n; i++ {
		labels = append(labels, render.TextLabel{Text: "x"})
	}
	return []runtime.TextBatchUpdate{{ID: id, Op: op, Labels: labels}}
}

func canvasTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func TestCanvasBatchReplaceAndDelete(t *testing.T) {
	var buf bytes.Buffer
	log := canvasTestLogger(&buf)
	var r canvasBatchRegistry

	r.applySprites(log, spriteUpdates(1, runtime.BatchReplace, 3))
	if got := len(r.sprites[1]); got != 3 {
		t.Fatalf("instances = %d, want 3", got)
	}

	// Replace is wholesale.
	r.applySprites(log, spriteUpdates(1, runtime.BatchReplace, 1))
	if got := len(r.sprites[1]); got != 1 {
		t.Fatalf("instances after re-replace = %d, want 1", got)
	}

	// Empty replace keeps an empty batch; it is not a delete.
	r.applySprites(log, spriteUpdates(1, runtime.BatchReplace, 0))
	if got := len(r.sprites[1]); got != 0 {
		t.Fatalf("instances after empty replace = %d, want 0", got)
	}
	if _, ok := r.sprites[1]; !ok {
		t.Fatal("empty replace removed the batch; only delete may remove it")
	}

	r.applySprites(log, spriteUpdates(1, runtime.BatchDelete, 0))
	if _, ok := r.sprites[1]; ok {
		t.Fatal("delete left the batch registered")
	}

	r.applyText(log, textUpdates(2, runtime.BatchReplace, 2))
	if got := len(r.text[2]); got != 2 {
		t.Fatalf("labels = %d, want 2", got)
	}
	r.applyText(log, textUpdates(2, runtime.BatchDelete, 0))
	if _, ok := r.text[2]; ok {
		t.Fatal("delete left the text batch registered")
	}

	if buf.Len() != 0 {
		t.Fatalf("no warnings expected below the caps; log:\n%s", buf.String())
	}
}

func TestCanvasBatchSpriteCapTruncatesWithOneWarning(t *testing.T) {
	var buf bytes.Buffer
	log := canvasTestLogger(&buf)
	var r canvasBatchRegistry

	r.applySprites(log, spriteUpdates(1, runtime.BatchReplace, canvasSpriteBatchCap+10))
	if got := len(r.sprites[1]); got != canvasSpriteBatchCap {
		t.Fatalf("instances = %d, want cap %d", got, canvasSpriteBatchCap)
	}

	// A second over-cap replace of the same batch must not warn again...
	r.applySprites(log, spriteUpdates(1, runtime.BatchReplace, canvasSpriteBatchCap+20))
	// ...but a different batch warns for itself.
	r.applySprites(log, spriteUpdates(2, runtime.BatchReplace, canvasSpriteBatchCap+1))

	if count := strings.Count(buf.String(), "level=WARN"); count != 2 {
		t.Fatalf("cap warnings = %d, want 2 (one per batch); log:\n%s", count, buf.String())
	}
}

func TestCanvasBatchTextCapTruncatesWithOneWarning(t *testing.T) {
	var buf bytes.Buffer
	log := canvasTestLogger(&buf)
	var r canvasBatchRegistry

	r.applyText(log, textUpdates(1, runtime.BatchReplace, canvasTextBatchCap+5))
	if got := len(r.text[1]); got != canvasTextBatchCap {
		t.Fatalf("labels = %d, want cap %d", got, canvasTextBatchCap)
	}
	r.applyText(log, textUpdates(1, runtime.BatchReplace, canvasTextBatchCap+6))

	if count := strings.Count(buf.String(), "level=WARN"); count != 1 {
		t.Fatalf("cap warnings = %d, want 1; log:\n%s", count, buf.String())
	}
}

func TestCanvasBatchAtCapDoesNotWarn(t *testing.T) {
	var buf bytes.Buffer
	log := canvasTestLogger(&buf)
	var r canvasBatchRegistry

	r.applySprites(log, spriteUpdates(1, runtime.BatchReplace, canvasSpriteBatchCap))
	r.applyText(log, textUpdates(1, runtime.BatchReplace, canvasTextBatchCap))

	if buf.Len() != 0 {
		t.Fatalf("no warnings expected exactly at the caps; log:\n%s", buf.String())
	}
	if got := len(r.sprites[1]); got != canvasSpriteBatchCap {
		t.Fatalf("instances = %d, want %d", got, canvasSpriteBatchCap)
	}
	if got := len(r.text[1]); got != canvasTextBatchCap {
		t.Fatalf("labels = %d, want %d", got, canvasTextBatchCap)
	}
}

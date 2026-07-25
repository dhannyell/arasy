package runtime_test

import (
	"bytes"
	"image"
	"image/color"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// dynamicManifest declares two dynamic textures (no static textures), 2x2
// each, so IDs are assigned in lexicographic name order: heatmap=1, plasma=2.
func dynamicManifest(t *testing.T) asset.Manifest {
	t.Helper()
	m, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{
			"version": 1,
			"dynamic": [
				{"name": "heatmap", "w": 2, "h": 2},
				{"name": "plasma", "w": 2, "h": 2}
			]
		}`)},
	})
	if err != nil {
		t.Fatalf("asset.Load: %v", err)
	}
	return m
}

func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// textureUpdateGame calls updateFn (if set) on every Render, letting tests
// control exactly what UpdateTexture calls happen in a given frame.
type textureUpdateGame struct {
	assets     asset.Manifest
	historyLen int
	updateFn   func(ctx *runtime.RenderContext)
}

func (g *textureUpdateGame) Setup() runtime.Config {
	return runtime.Config{Assets: g.assets, HistoryLen: g.historyLen}
}
func (g *textureUpdateGame) Init(ctx *runtime.UpdateContext)               {}
func (g *textureUpdateGame) Update(ctx *runtime.UpdateContext, dt float64) {}
func (g *textureUpdateGame) Render(ctx *runtime.RenderContext) []render.Command {
	if g.updateFn != nil {
		g.updateFn(ctx)
	}
	return nil
}

func TestUpdateTexturePublishConsume(t *testing.T) {
	m := dynamicManifest(t)
	heatmap := m.Texture("heatmap")
	img := solidImage(2, 2, color.RGBA{R: 200, A: 255})

	game := &textureUpdateGame{assets: m}
	driver := runtime.NewDriver(game, testLogger())
	game.updateFn = func(ctx *runtime.RenderContext) { ctx.UpdateTexture(heatmap, img) }

	driver.Frame(0)

	updates := driver.ConsumeTextureUpdates()
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(updates))
	}
	if updates[0].ID != heatmap {
		t.Fatalf("id = %d, want %d", updates[0].ID, heatmap)
	}
	if got := updates[0].Image.RGBAAt(0, 0); got != (color.RGBA{R: 200, A: 255}) {
		t.Fatalf("pixel = %+v, want {200 0 0 255}", got)
	}

	if more := driver.ConsumeTextureUpdates(); len(more) != 0 {
		t.Fatalf("second consume = %d, want 0 (pending must be drained)", len(more))
	}
}

func TestUpdateTextureDimensionMismatchDropped(t *testing.T) {
	m := dynamicManifest(t)
	heatmap := m.Texture("heatmap")
	badImg := image.NewRGBA(image.Rect(0, 0, 3, 3)) // declared 2x2

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	game := &textureUpdateGame{assets: m}
	driver := runtime.NewDriver(game, log)
	game.updateFn = func(ctx *runtime.RenderContext) { ctx.UpdateTexture(heatmap, badImg) }

	driver.Frame(0)

	if updates := driver.ConsumeTextureUpdates(); len(updates) != 0 {
		t.Fatalf("updates = %d, want 0 for dimension mismatch", len(updates))
	}
	if !strings.Contains(buf.String(), "dimension mismatch") {
		t.Fatalf("expected dimension mismatch warning, got log: %s", buf.String())
	}
}

func TestUpdateTextureUnknownIDDropped(t *testing.T) {
	m := dynamicManifest(t)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	game := &textureUpdateGame{assets: m}
	driver := runtime.NewDriver(game, log)
	unknown := render.TextureID(999)
	game.updateFn = func(ctx *runtime.RenderContext) {
		ctx.UpdateTexture(unknown, solidImage(2, 2, color.RGBA{A: 255}))
	}

	driver.Frame(0)

	if updates := driver.ConsumeTextureUpdates(); len(updates) != 0 {
		t.Fatalf("updates = %d, want 0 for unknown id", len(updates))
	}
	if !strings.Contains(buf.String(), "unknown or non-dynamic") {
		t.Fatalf("expected unknown/non-dynamic warning, got log: %s", buf.String())
	}
}

func TestUpdateTextureDuplicateInFrameKeepsLast(t *testing.T) {
	m := dynamicManifest(t)
	heatmap := m.Texture("heatmap")
	first := solidImage(2, 2, color.RGBA{R: 10, A: 255})
	last := solidImage(2, 2, color.RGBA{R: 250, A: 255})

	game := &textureUpdateGame{assets: m}
	driver := runtime.NewDriver(game, testLogger())
	game.updateFn = func(ctx *runtime.RenderContext) {
		ctx.UpdateTexture(heatmap, first)
		ctx.UpdateTexture(heatmap, last)
	}

	driver.Frame(0)

	updates := driver.ConsumeTextureUpdates()
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want 1 (same-frame duplicates collapse)", len(updates))
	}
	if got := updates[0].Image.RGBAAt(0, 0); got != (color.RGBA{R: 250, A: 255}) {
		t.Fatalf("pixel = %+v, want last write {250 0 0 255}", got)
	}
}

func TestUpdateTextureConsumeSortedByID(t *testing.T) {
	m := dynamicManifest(t)
	heatmap := m.Texture("heatmap")
	plasma := m.Texture("plasma")
	if heatmap >= plasma {
		t.Fatalf("test assumes heatmap(%d) < plasma(%d)", heatmap, plasma)
	}

	game := &textureUpdateGame{assets: m}
	driver := runtime.NewDriver(game, testLogger())
	game.updateFn = func(ctx *runtime.RenderContext) {
		ctx.UpdateTexture(plasma, solidImage(2, 2, color.RGBA{G: 1, A: 255}))
		ctx.UpdateTexture(heatmap, solidImage(2, 2, color.RGBA{R: 1, A: 255}))
	}

	driver.Frame(0)

	updates := driver.ConsumeTextureUpdates()
	if len(updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(updates))
	}
	if updates[0].ID != heatmap || updates[1].ID != plasma {
		t.Fatalf("order = [%d, %d], want [%d, %d]", updates[0].ID, updates[1].ID, heatmap, plasma)
	}
}

// TestUpdateTexturePendingSurvivesRewind guards the render-only boundary:
// texture updates must never be part of the rewind snapshot, so a Back
// between publish and consume must neither drop nor duplicate them.
func TestUpdateTexturePendingSurvivesRewind(t *testing.T) {
	m := dynamicManifest(t)
	heatmap := m.Texture("heatmap")

	game := &textureUpdateGame{assets: m, historyLen: 4}
	driver := runtime.NewDriver(game, testLogger())
	driver.Step()
	driver.Step()

	game.updateFn = func(ctx *runtime.RenderContext) {
		ctx.UpdateTexture(heatmap, solidImage(2, 2, color.RGBA{R: 5, A: 255}))
	}
	driver.Frame(0)

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlBack})
	driver.AdvanceTime(0)

	updates := driver.ConsumeTextureUpdates()
	if len(updates) != 1 {
		t.Fatalf("pending updates after rewind = %d, want 1 (render-only state must survive rewind untouched)", len(updates))
	}
}

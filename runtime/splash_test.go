package runtime

import (
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/enginesplash"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type splashGame struct {
	cfg Config
}

func (g splashGame) Setup() Config                            { return g.cfg }
func (splashGame) Init(ctx *UpdateContext)                    {}
func (splashGame) Update(ctx *UpdateContext, dt float64)      {}
func (splashGame) Render(ctx *RenderContext) []render.Command { return nil }

type splashCaptureRenderer struct {
	frames []Frame
}

func (r *splashCaptureRenderer) UpdateResources(FrameUpdates) {}
func (r *splashCaptureRenderer) Render(f Frame)               { r.frames = append(r.frames, f) }

func newTestDriver(t *testing.T, cfg Config) *Driver {
	t.Helper()
	return NewDriver(splashGame{cfg: cfg}, slog.Default())
}

// mustSplashManifest loads a manifest from an in-memory manifest.json,
// synthesizing empty files for every "file" it references (Load stores raw
// bytes verbatim without decoding images).
func mustSplashManifest(t *testing.T, manifestJSON []byte) asset.Manifest {
	t.Helper()
	fsys := fstest.MapFS{
		"manifest.json": {Data: manifestJSON},
		"a.png":         {Data: []byte{0}},
		"b.png":         {Data: []byte{0}},
	}
	m, err := asset.Load(fsys)
	if err != nil {
		t.Fatalf("asset.Load: %v", err)
	}
	return m
}

func TestSplashAdvancesThroughSegments(t *testing.T) {
	d := newTestDriver(t, Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "a", "file": "a.png"}, {"name": "b", "file": "b.png"}],
			"splash": [
				{"texture": "a", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 1},
				{"texture": "b", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 1}
			]
		}`)),
	})

	if !d.SplashActive() {
		t.Fatal("SplashActive() = false, want true at boot")
	}

	d.AdvanceSplash(500*time.Millisecond, false)
	if d.splash.index != 0 {
		t.Fatalf("index = %d after 0.5s, want still on segment 0", d.splash.index)
	}

	d.AdvanceSplash(600*time.Millisecond, false)
	if d.splash.index != 1 {
		t.Fatalf("index = %d after passing MinTime, want 1", d.splash.index)
	}

	d.AdvanceSplash(1100*time.Millisecond, false)
	if d.SplashActive() {
		t.Fatal("SplashActive() = true after the last segment's MinTime elapsed, want false")
	}
}

func TestSkippableEndsSegmentOnInput(t *testing.T) {
	skippable := Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "a", "file": "a.png"}],
			"splash": [{"texture": "a", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 10, "skippable": true}]
		}`)),
	}
	d := newTestDriver(t, skippable)
	d.AdvanceSplash(10*time.Millisecond, true)
	if d.SplashActive() {
		t.Fatal("Skippable segment did not end on input before MinTime")
	}

	notSkippable := Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "a", "file": "a.png"}],
			"splash": [{"texture": "a", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 10, "skippable": false}]
		}`)),
	}
	d2 := newTestDriver(t, notSkippable)
	d2.AdvanceSplash(10*time.Millisecond, true)
	if !d2.SplashActive() {
		t.Fatal("non-Skippable segment ended on input before MinTime")
	}
}

func TestSplashNeverAdvancesTick(t *testing.T) {
	d := newTestDriver(t, Config{DisableDefaultSplash: false})

	for range 5 {
		d.AdvanceSplash(500*time.Millisecond, false)
	}
	for d.SplashActive() {
		d.AdvanceSplash(500*time.Millisecond, false)
	}

	state := d.State()
	if state.Tick != 0 {
		t.Fatalf("Tick = %d after draining the splash, want 0", state.Tick)
	}
	if state.HistoryLen != 0 {
		t.Fatalf("HistoryLen = %d after draining the splash, want 0 (no recorder anchors)", state.HistoryLen)
	}
}

func TestSplashDisabledSkipsEngineSegment(t *testing.T) {
	d := newTestDriver(t, Config{DisableDefaultSplash: true})
	if d.SplashActive() {
		t.Fatal("SplashActive() = true with DisableDefaultSplash and no manifest segments, want false")
	}
}

func TestSplashFrameEmitsDrawImage(t *testing.T) {
	d := newTestDriver(t, Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "a", "file": "a.png"}],
			"splash": [{"texture": "a", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 1, "fade": 0.2}]
		}`)),
	})
	d.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}))

	r := &splashCaptureRenderer{}
	d.RenderSplashFrame(0, r)
	frame := r.frames[0]

	var found *render.DrawImage
	for i := range frame.Commands {
		if img, ok := frame.Commands[i].(render.DrawImage); ok {
			found = &img
		}
	}
	if found == nil {
		t.Fatal("RenderSplashFrame did not emit a DrawImage for a texture segment")
	}

	d.AdvanceSplash(100*time.Millisecond, false) // inside the 0.2s fade-in
	r2 := &splashCaptureRenderer{}
	d.RenderSplashFrame(0, r2)
	var mid *render.DrawImage
	for i := range r2.frames[0].Commands {
		if img, ok := r2.frames[0].Commands[i].(render.DrawImage); ok {
			mid = &img
		}
	}
	if mid == nil || mid.Tint.A >= 1 {
		t.Fatalf("expected alpha < 1 mid fade-in, got %+v", mid)
	}
}

func TestOfflinePathHasNoSplash(t *testing.T) {
	d := newTestDriver(t, Config{DisableDefaultSplash: false})
	d.Step()
	if d.State().Tick != 1 {
		t.Fatalf("Tick = %d after one Step without draining the splash, want 1", d.State().Tick)
	}
}

func TestEngineSplashRendersWithoutManifest(t *testing.T) {
	d := newTestDriver(t, Config{})
	if !d.SplashActive() {
		t.Fatal("SplashActive() = false, want true (engine segment enabled by default)")
	}
	d.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}))

	r := &splashCaptureRenderer{}
	d.RenderSplashFrame(0, r)
	if len(r.frames[0].Commands) == 0 {
		t.Fatal("engine splash segment produced no commands")
	}
}

// TestEngineSplashBackgroundMatchesBrandOnTallViewport guards the seam this
// was meant to fix: on a viewport far from the compiled animation's own
// 16:9, the letterboxed area is small (tall phone screens), so the outer
// background must match the animation's own painted backdrop instead of an
// unrelated Config.Clear, or the two show up as a visible box.
func TestEngineSplashBackgroundMatchesBrandOnTallViewport(t *testing.T) {
	d := newTestDriver(t, Config{Clear: render.RGBA8(200, 30, 30, 255)})
	d.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 390, Y: 844}))

	r := &splashCaptureRenderer{}
	d.RenderSplashFrame(0, r)

	rect, ok := r.frames[0].Commands[0].(render.DrawRect)
	if !ok {
		t.Fatalf("Commands[0] = %T, want the background render.DrawRect", r.frames[0].Commands[0])
	}
	if rect.Fill != enginesplash.EngineSplashBackground {
		t.Fatalf("background = %+v, want the brand backdrop %+v (not Config.Clear)", rect.Fill, enginesplash.EngineSplashBackground)
	}
}

func TestSplashFadeAlphaEnvelope(t *testing.T) {
	const minTime, fade = 1.0, 0.25
	if a := splashFadeAlpha(0, minTime, fade); a != 0 {
		t.Fatalf("alpha at t=0 = %v, want 0", a)
	}
	if a := splashFadeAlpha(fade/2, minTime, fade); a <= 0 || a >= 1 {
		t.Fatalf("alpha mid fade-in = %v, want in (0,1)", a)
	}
	if a := splashFadeAlpha(minTime/2, minTime, fade); a != 1 {
		t.Fatalf("alpha at plateau = %v, want 1", a)
	}
	if a := splashFadeAlpha(minTime-fade/2, minTime, fade); a <= 0 || a >= 1 {
		t.Fatalf("alpha mid fade-out = %v, want in (0,1)", a)
	}
	if a := splashFadeAlpha(0, minTime, 0); a != 1 {
		t.Fatalf("alpha with Fade=0 = %v, want 1 (hard cut)", a)
	}
}

func TestFlipBookCellByRenderClock(t *testing.T) {
	seg := SplashSegment{CellW: 10, CellH: 10, Columns: 2, Frames: 4, Rate: 2}
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})

	img0 := splashFlipbookImage(seg, 0, viewport)
	img1 := splashFlipbookImage(seg, 0.5, viewport) // 1 frame elapsed at rate=2
	if img0.Src == img1.Src {
		t.Fatalf("expected the flip-book cell to advance, got the same Src %+v", img0.Src)
	}
}

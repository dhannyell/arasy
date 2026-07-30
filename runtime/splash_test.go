package runtime

import (
	"bytes"
	"image"
	"image/png"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
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
// synthesizing a valid texture large enough for the test flip-books.
func mustSplashManifest(t *testing.T, manifestJSON []byte) asset.Manifest {
	t.Helper()
	var texture bytes.Buffer
	if err := png.Encode(&texture, image.NewNRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	fsys := fstest.MapFS{
		"manifest.json": {Data: manifestJSON},
		"a.png":         {Data: texture.Bytes()},
		"b.png":         {Data: texture.Bytes()},
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

	if !d.splashActive() {
		t.Fatal("splashActive() = false, want true at boot")
	}

	d.advanceSplash(500*time.Millisecond, false)
	if d.splash.index != 0 {
		t.Fatalf("index = %d after 0.5s, want still on segment 0", d.splash.index)
	}

	d.advanceSplash(600*time.Millisecond, false)
	if d.splash.index != 1 {
		t.Fatalf("index = %d after passing MinTime, want 1", d.splash.index)
	}

	d.advanceSplash(1100*time.Millisecond, false)
	if d.splashActive() {
		t.Fatal("splashActive() = true after the last segment's MinTime elapsed, want false")
	}
}

func TestSkippableEndsSegmentOnActivation(t *testing.T) {
	skippable := Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "a", "file": "a.png"}],
			"splash": [{"texture": "a", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 10, "skippable": true}]
		}`)),
	}
	d := newTestDriver(t, skippable)
	renderer := &splashCaptureRenderer{}
	result := d.RunSplashFrame(SplashFrameBatch{
		Events: []input.Event{{Kind: input.EventButtonDown}},
	}, 10*time.Millisecond, renderer)
	if !result.Rendered || len(renderer.frames) != 1 {
		t.Fatalf("RunSplashFrame() = %+v with %d frames, want one rendered splash frame", result, len(renderer.frames))
	}
	if d.splashActive() {
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
	d2.RunSplashFrame(SplashFrameBatch{
		Events: []input.Event{{Kind: input.EventButtonDown}},
	}, 10*time.Millisecond, &splashCaptureRenderer{})
	if !d2.splashActive() {
		t.Fatal("non-Skippable segment ended on input before MinTime")
	}
}

func TestSplashNeverAdvancesTick(t *testing.T) {
	d := newTestDriver(t, Config{DisableDefaultSplash: false})
	renderer := &splashCaptureRenderer{}

	for {
		result := d.RunSplashFrame(SplashFrameBatch{}, 500*time.Millisecond, renderer)
		if !result.Rendered {
			break
		}
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
	renderer := &splashCaptureRenderer{}
	if result := d.RunSplashFrame(SplashFrameBatch{}, 0, renderer); result != (SplashFrameResult{}) {
		t.Fatalf("RunSplashFrame() = %+v with disabled splash, want zero result", result)
	}
	if len(renderer.frames) != 0 {
		t.Fatalf("disabled splash rendered %d frames, want 0", len(renderer.frames))
	}
}

func TestSkipDefaultSplashPreservesManifestSegments(t *testing.T) {
	d := newTestDriver(t, Config{
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "studio", "file": "a.png"}],
			"splash": [{
				"texture": "studio",
				"cell_w": 1,
				"cell_h": 1,
				"columns": 1,
				"frames": 1,
				"min_time": 1
			}]
		}`)),
	})

	d.SkipDefaultSplash()
	if !d.splashActive() {
		t.Fatal("splashActive() = false after skipping the default segment; manifest segment was removed")
	}

	d.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}))
	frame := d.splashFrame(0)
	var image *render.DrawImage
	for _, command := range frame.Commands {
		if draw, ok := command.(render.DrawImage); ok {
			image = &draw
			break
		}
	}
	if image == nil || image.Texture == 0 {
		t.Fatalf("manifest segment frame = %+v, want its resolved texture", image)
	}
}

func TestSkipDefaultSplashAfterItPlayedPreservesManifestElapsed(t *testing.T) {
	d := newTestDriver(t, Config{
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "studio", "file": "a.png"}],
			"splash": [{
				"texture": "studio",
				"cell_w": 1,
				"cell_h": 1,
				"columns": 1,
				"frames": 1,
				"min_time": 2
			}]
		}`)),
	})

	d.advanceSplash(time.Duration(enginesplash.MinTime*float64(time.Second)), false)
	d.advanceSplash(500*time.Millisecond, false)
	d.SkipDefaultSplash()

	if !d.splashActive() {
		t.Fatal("manifest segment was removed after the default segment had already played")
	}
	if got := d.splash.elapsed; got != 0.5 {
		t.Fatalf("manifest elapsed = %v, want 0.5", got)
	}
}

func TestSkipSplashRemovesDefaultAndManifestSegmentsWithoutAdvancingTick(t *testing.T) {
	d := newTestDriver(t, Config{
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "a", "file": "a.png"}],
			"splash": [{"texture": "a", "cell_w": 4, "cell_h": 4, "columns": 1, "frames": 1, "min_time": 10}]
		}`)),
	})
	if !d.splashActive() {
		t.Fatal("test setup has no splash to skip")
	}

	d.SkipSplash()

	if d.splashActive() {
		t.Fatal("splashActive() = true after SkipSplash")
	}
	if got := d.State().Tick; got != 0 {
		t.Fatalf("Tick = %d after SkipSplash, want 0", got)
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
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})
	r := &splashCaptureRenderer{}
	d.RunSplashFrame(SplashFrameBatch{Viewport: &viewport}, 0, r)
	frame := r.frames[0]

	var found *render.DrawImage
	for i := range frame.Commands {
		if img, ok := frame.Commands[i].(render.DrawImage); ok {
			found = &img
		}
	}
	if found == nil {
		t.Fatal("renderSplashFrame did not emit a DrawImage for a texture segment")
	}

	r2 := &splashCaptureRenderer{}
	d.RunSplashFrame(SplashFrameBatch{}, 100*time.Millisecond, r2) // inside the 0.2s fade-in
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
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080})
	r := &splashCaptureRenderer{}
	result := d.RunSplashFrame(SplashFrameBatch{Viewport: &viewport}, 0, r)
	if !result.Rendered {
		t.Fatal("RunSplashFrame() did not render the default engine segment")
	}
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
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 390, Y: 844})
	r := &splashCaptureRenderer{}
	d.RunSplashFrame(SplashFrameBatch{Viewport: &viewport}, 0, r)

	rect, ok := r.frames[0].Commands[0].(render.DrawRect)
	if !ok {
		t.Fatalf("Commands[0] = %T, want the background render.DrawRect", r.frames[0].Commands[0])
	}
	if rect.Fill != enginesplash.Background {
		t.Fatalf("background = %+v, want the brand backdrop %+v (not Config.Clear)", rect.Fill, enginesplash.Background)
	}
}

func TestManifestSplashUsesBootBackgroundThenClearFallback(t *testing.T) {
	manifest := mustSplashManifest(t, []byte(`{
		"version": 1,
		"textures": [{"name": "studio", "file": "a.png"}],
		"splash": [{
			"texture": "studio",
			"cell_w": 1,
			"cell_h": 1,
			"columns": 1,
			"frames": 1,
			"min_time": 1
		}]
	}`))
	clear := render.RGBA8(10, 20, 30, 255)
	boot := render.RGBA8(40, 50, 60, 255)

	for _, tc := range []struct {
		name string
		cfg  Config
		want render.Color
	}{
		{
			name: "configured boot background",
			cfg: Config{
				Assets:               manifest,
				Clear:                clear,
				BootBackground:       boot,
				DisableDefaultSplash: true,
			},
			want: boot,
		},
		{
			name: "clear fallback",
			cfg: Config{
				Assets:               manifest,
				Clear:                clear,
				DisableDefaultSplash: true,
			},
			want: clear,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDriver(t, tc.cfg)
			d.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}))
			frame := d.splashFrame(0)
			background, ok := frame.Commands[0].(render.DrawRect)
			if !ok {
				t.Fatalf("Commands[0] = %T, want render.DrawRect", frame.Commands[0])
			}
			if background.Fill != tc.want {
				t.Fatalf("background = %+v, want %+v", background.Fill, tc.want)
			}
		})
	}
}

func TestManifestSplashParticlesUseSegmentScopedIdentity(t *testing.T) {
	manifest := mustSplashManifest(t, []byte(`{
		"version": 1,
		"textures": [{"name": "studio", "file": "a.png"}],
		"effects": [{
			"name": "dust",
			"capacity": 8,
			"lifetime": [1, 1],
			"size_start": [1, 1]
		}],
		"splash": [
			{
				"texture": "studio",
				"cell_w": 1,
				"cell_h": 1,
				"columns": 1,
				"frames": 1,
				"effect": "dust"
			},
			{
				"texture": "studio",
				"cell_w": 1,
				"cell_h": 1,
				"columns": 1,
				"frames": 1,
				"effect": "dust"
			}
		]
	}`))

	segments := buildSplashPlaylist(Config{
		Assets:               manifest,
		DisableDefaultSplash: true,
	})
	if len(segments) != 2 {
		t.Fatalf("buildSplashPlaylist() len = %d, want 2", len(segments))
	}

	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})
	first := segments[0].Particles(0, 1.0/60, viewport, 1)
	second := segments[1].Particles(0, 1.0/60, viewport, 1)
	firstAgain := segments[0].Particles(0.5, 1.0/60, viewport, 1)
	if len(first) != 1 || len(second) != 1 || len(firstAgain) != 1 {
		t.Fatalf("particle command counts = %d, %d, %d; want one each", len(first), len(second), len(firstAgain))
	}

	if first[0].Effect != second[0].Effect {
		t.Fatalf("effects = %d and %d, want the shared manifest effect", first[0].Effect, second[0].Effect)
	}
	if first[0].Emitter == second[0].Emitter {
		t.Fatalf("emitters = %d and %d, want segment-scoped identities", first[0].Emitter, second[0].Emitter)
	}
	if first[0].Seed == second[0].Seed {
		t.Fatalf("seeds = %d and %d, want segment-scoped identities", first[0].Seed, second[0].Seed)
	}
	if first[0].Emitter != firstAgain[0].Emitter || first[0].Seed != firstAgain[0].Seed {
		t.Fatalf(
			"first segment identity changed across frames: (%d, %d) then (%d, %d)",
			first[0].Emitter,
			first[0].Seed,
			firstAgain[0].Emitter,
			firstAgain[0].Seed,
		)
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

func TestRunSplashFrameWaitsForFullOpacityBeforeHandoff(t *testing.T) {
	d := newTestDriver(t, Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "studio", "file": "a.png"}],
			"splash": [{
				"texture": "studio",
				"cell_w": 1,
				"cell_h": 1,
				"columns": 1,
				"frames": 1,
				"min_time": 1,
				"fade": 0.25
			}]
		}`)),
	})

	renderer := &splashCaptureRenderer{}
	result := d.RunSplashFrame(SplashFrameBatch{}, 0, renderer)
	if !result.Rendered || result.ReadyForHandoff {
		t.Fatalf("RunSplashFrame() at transparent start = %+v, want rendered without handoff", result)
	}
	result = d.RunSplashFrame(SplashFrameBatch{}, 125*time.Millisecond, renderer)
	if !result.Rendered || result.ReadyForHandoff {
		t.Fatalf("RunSplashFrame() during fade-in = %+v, want rendered without handoff", result)
	}
	result = d.RunSplashFrame(SplashFrameBatch{}, 125*time.Millisecond, renderer)
	if !result.Rendered || !result.ReadyForHandoff {
		t.Fatalf("RunSplashFrame() at full opacity = %+v, want handoff ready", result)
	}
	result = d.RunSplashFrame(SplashFrameBatch{}, 750*time.Millisecond, renderer)
	if !result.Rendered || result.ReadyForHandoff {
		t.Fatalf("RunSplashFrame() after playlist end = %+v, want final splash frame without handoff", result)
	}
	if result := d.RunSplashFrame(SplashFrameBatch{}, 0, renderer); result != (SplashFrameResult{}) {
		t.Fatalf("RunSplashFrame() after splash drained = %+v, want zero result", result)
	}
}

func TestRunSplashFrameAppliesViewportAndOnlyActivationSkips(t *testing.T) {
	d := newTestDriver(t, Config{
		DisableDefaultSplash: true,
		Assets: mustSplashManifest(t, []byte(`{
			"version": 1,
			"textures": [{"name": "studio", "file": "a.png"}],
			"splash": [{
				"texture": "studio",
				"cell_w": 1,
				"cell_h": 1,
				"columns": 1,
				"frames": 1,
				"min_time": 10,
				"skippable": true
			}]
		}`)),
	})
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180})
	renderer := &splashCaptureRenderer{}

	result := d.RunSplashFrame(SplashFrameBatch{
		Viewport: &viewport,
		Events:   []input.Event{{Kind: input.EventPointerMove}},
	}, 10*time.Millisecond, renderer)
	if !result.Rendered {
		t.Fatal("RunSplashFrame() did not render the active splash")
	}
	if !d.splashActive() {
		t.Fatal("passive pointer motion skipped a skippable splash")
	}
	if got := renderer.frames[0].Camera.Viewport; got != viewport {
		t.Fatalf("rendered viewport = %+v, want %+v", got, viewport)
	}

	d.RunSplashFrame(SplashFrameBatch{
		Events: []input.Event{{Kind: input.EventButtonDown}},
	}, 10*time.Millisecond, renderer)
	if d.splashActive() {
		t.Fatal("activation input did not skip a skippable splash")
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

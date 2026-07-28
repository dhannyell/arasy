package tess

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font/gofont/goregular"
)

func TestNewAtlasForFacePacksASCIIAndRequestedUnicode(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	atlas, err := NewAtlasForFace(3, 18, face, []rune{'Á', '界', 'Á'})
	if err != nil {
		t.Fatal(err)
	}
	if atlas.Key != (AtlasKey{Font: 3, PX: 18}) {
		t.Fatalf("atlas.Key = %+v", atlas.Key)
	}
	if atlas.Image.Bounds().Dx() <= 0 || atlas.Image.Bounds().Dy() <= 0 {
		t.Fatalf("invalid atlas bounds %v", atlas.Image.Bounds())
	}
	if atlas.Ascent <= 0 || atlas.Height <= atlas.Ascent {
		t.Fatalf("invalid metrics ascent=%v height=%v", atlas.Ascent, atlas.Height)
	}
	for _, r := range []rune{' ', '?', 'A', 'Á'} {
		glyph, ok := atlas.Glyphs[r]
		if !ok {
			t.Fatalf("atlas is missing %q", r)
		}
		if glyph.Advance <= 0 {
			t.Fatalf("glyph %q advance = %v", r, glyph.Advance)
		}
	}
	if _, ok := atlas.Glyphs['界']; ok {
		t.Fatal("Go Regular unexpectedly exposed unsupported rune instead of '?' fallback")
	}
	if !hasNonZeroAlpha(atlas.Image.Pix) {
		t.Fatal("external atlas is empty")
	}

	// Empty input remains useful because printable ASCII is always included.
	if _, err := NewAtlasForFace(3, 18, face, nil); err != nil {
		t.Fatalf("empty rune input: %v", err)
	}
}

func TestNewAtlasForFaceRejectsInvalidInputs(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAtlasForFace(1, 0, face, nil); err == nil {
		t.Fatal("zero pixel size succeeded")
	}
	if _, err := NewAtlasForFace(1, 12, nil, nil); err == nil {
		t.Fatal("nil face succeeded")
	}
}

func TestMaximumRasterizedAtlasFitsWebGPUMinimumLimit(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := NewAtlasForFace(1, maxAtlasPixelSize, face, nil)
	if err != nil {
		t.Fatal(err)
	}
	bounds := atlas.Image.Bounds()
	if bounds.Dx() > maxAtlasDimension || bounds.Dy() > maxAtlasDimension {
		t.Fatalf("atlas dimensions=%dx%d exceed %d", bounds.Dx(), bounds.Dy(), maxAtlasDimension)
	}
}

func TestIndexedASCIIGlyphsUsesQuestionMarkForUnsupportedGlyph(t *testing.T) {
	fallback := Glyph{Advance: 7, Width: 5, Height: 9, index: 3}
	letter := Glyph{Advance: 8, Width: 6, Height: 9, index: 4}
	indexed := indexedASCIIGlyphs(map[rune]Glyph{'?': fallback, 'A': letter})
	if got := indexed['A']; got != letter {
		t.Fatalf("indexed A = %+v, want %+v", got, letter)
	}
	if got := indexed['B']; got != fallback {
		t.Fatalf("indexed unsupported B = %+v, want question-mark fallback %+v", got, fallback)
	}
}

func TestBuildExternalTypefaceUsesProportionalAdvances(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	narrow := buildTextRun(t, provider, "iiii", 1)
	wide := buildTextRun(t, provider, "MMMM", 1)

	if textMeshWidth(narrow.Text) >= textMeshWidth(wide.Text) {
		t.Fatalf("iiii width=%v, want less than MMMM width=%v", textMeshWidth(narrow.Text), textMeshWidth(wide.Text))
	}
	if narrow.AtlasKey.Font != 1 || narrow.AtlasKey.PX != 18 {
		t.Fatalf("external run atlas = %+v", narrow.AtlasKey)
	}
}

func TestBuildUnknownTypefaceFallsBackToFontZero(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	run := buildTextRun(t, provider, "fallback", 99)
	if run.AtlasKey.Font != 0 {
		t.Fatalf("fallback atlas = %+v, want Font 0", run.AtlasKey)
	}
	if len(run.Text.Vertices) == 0 {
		t.Fatal("fallback emitted no text")
	}
}

func TestBuildFontZeroKeepsCanonicalTextWidth(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	run := buildTextRun(t, provider, "mono", 0)
	wantScreenWidth := render.MeasureText("mono", 18).X
	wantClipWidth := wantScreenWidth * 2 / 100
	if got := textMeshWidth(run.Text); math.Abs(got-wantClipWidth) > testEpsilon {
		t.Fatalf("Font 0 clip width=%v, want canonical %v", got, wantClipWidth)
	}
}

func TestDefaultFaceAtlasPreservesNarrowGlyphBounds(t *testing.T) {
	face := typeface.DefaultFace()
	if face == nil {
		t.Skip("default font compiled out (arr_no_default_font)")
	}

	atlas, err := NewDefaultFaceAtlas(18, face, []rune{'!'})
	if err != nil {
		t.Fatal(err)
	}
	if !atlas.LinearFilter {
		t.Fatal("default TTF atlas must use linear filtering")
	}
	glyph := atlas.glyph('!')
	if glyph.Width <= 0 || glyph.Height <= 0 {
		t.Fatalf("exclamation glyph bounds = %vx%v, want visible glyph", glyph.Width, glyph.Height)
	}
	if glyph.Width >= glyph.Advance {
		t.Fatalf("exclamation glyph width = %v, advance = %v; want a narrow glyph within its monospace cell", glyph.Width, glyph.Advance)
	}
	metric := atlas.metrics[glyph.index]
	if math.Abs(float64(metric.width)-glyph.Width) > testEpsilon {
		t.Fatalf("exclamation metric width = %v, glyph width = %v; visual bounds must not use the layout advance", metric.width, glyph.Width)
	}
}

func TestBuildTextRotationAndClipRemainFinite(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	var builder Builder
	runs := builder.Build(Frame{
		Camera: testCamera(),
		Commands: []render.Command{render.DrawText{
			BaseCommand: render.BaseCommand{Clip: math2d.NewRect(
				math2d.Vec2{X: 42, Y: 42}, math2d.Vec2{X: 58, Y: 58},
			)},
			Position: math2d.Vec2{X: 50, Y: 50},
			Text:     "Rotate", Color: render.RGBA(1, 1, 1, 1), Size: 18,
			Font: 1, Rotation: math.Pi / 5,
		}},
	}, 100, 100, provider, nil)
	if len(runs) != 1 || runs[0].VertexCount == 0 {
		t.Fatalf("rotated clipped text produced %+v", runs)
	}
	label := decodeTextLabel(t, builder.TextLabelStream(), 0)
	for _, pair := range [][2]float32{
		label.position, label.transformAB, label.transformCD,
		label.originBaseline, label.clipMin, label.clipMax,
	} {
		for _, value := range pair {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				t.Fatalf("non-finite label record %+v", label)
			}
		}
	}
}

func TestBuildSplitsTextRunsByAtlasWithoutChangingPainterOrder(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	var builder Builder
	runs := builder.Build(Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(0),
			textCommand(1, 1, "external"),
			textCommand(2, 0, "builtin"),
			textCommand(3, 1, "external again"),
			drawRect(4),
		},
	}, 100, 100, provider, nil)

	wantKinds := []DrawRunKind{DrawRunSolid, DrawRunText, DrawRunText, DrawRunText, DrawRunSolid}
	wantFonts := []render.FontID{0, 1, 0, 1, 0}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs)=%d, want %d: %+v", len(runs), len(wantKinds), runs)
	}
	for i := range runs {
		if runs[i].Kind != wantKinds[i] || runs[i].AtlasKey.Font != wantFonts[i] {
			t.Fatalf("run[%d] kind=%v atlas=%+v", i, runs[i].Kind, runs[i].AtlasKey)
		}
	}
}

func TestBuildRequestsAllFrameRunesOncePerAtlasKey(t *testing.T) {
	provider := &recordingProvider{testTypefaceProvider: *newTestTypefaceProvider(t)}
	var builder Builder
	runs := builder.Build(Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			textCommand(0, 1, "Á"),
			textCommand(0, 1, "é"),
		},
	}, 100, 100, provider, nil)
	if len(provider.calls) != 1 {
		t.Fatalf("provider calls=%d, want one per frame key", len(provider.calls))
	}
	if !strings.Contains(provider.calls[0], "Á") || !strings.Contains(provider.calls[0], "é") {
		t.Fatalf("provider text=%q, want both frame runes", provider.calls[0])
	}
	if len(runs) != 1 || runs[0].AtlasKey != (AtlasKey{Font: 1, PX: 18}) {
		t.Fatalf("same-key text was not kept in one ordered run: %+v", runs)
	}
}

func TestBuildUsesCachedASCIIAtlasAndStillRequestsNewUnicode(t *testing.T) {
	provider := newCachedRecordingProvider(t)
	frame := Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			textCommand(0, 1, "cached ASCII"),
		},
	}
	var builder Builder
	builder.Build(frame, 100, 100, provider, nil)
	if len(provider.calls) != 0 {
		t.Fatalf("cached ASCII caused AtlasFor calls: %v", provider.calls)
	}

	frame.Commands = []render.Command{textCommand(0, 1, "new Á")}
	builder.Build(frame, 100, 100, provider, nil)
	if len(provider.calls) != 1 || !strings.Contains(provider.calls[0], "Á") {
		t.Fatalf("Unicode AtlasFor calls = %v, want one request containing Á", provider.calls)
	}
}

func TestBuildTwoExternalFontsProduceSeparateAtlasRuns(t *testing.T) {
	provider := newMultiFontTypefaceProvider(t)
	var builder Builder
	runs := builder.Build(Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			textCommand(0, 1, "one"),
			textCommand(1, 2, "two"),
		},
	}, 100, 100, provider, nil)

	if len(runs) != 2 {
		t.Fatalf("len(runs)=%d, want 2 separate runs: %+v", len(runs), runs)
	}
	for i, run := range runs {
		if run.Kind != DrawRunText {
			t.Fatalf("run[%d].Kind = %v, want DrawRunText", i, run.Kind)
		}
	}
	if runs[0].AtlasKey.Font != 1 || runs[1].AtlasKey.Font != 2 {
		t.Fatalf("run fonts = %v, %v; want 1 then 2", runs[0].AtlasKey.Font, runs[1].AtlasKey.Font)
	}
}

// multiFontTypefaceProvider resolves several FontIDs to distinct real faces,
// exercising the multi-atlas path that a single-font provider can't reach.
type multiFontTypefaceProvider struct {
	faces map[render.FontID]*typeface.Face
}

func (p *multiFontTypefaceProvider) AtlasFor(fontID render.FontID, px int, text string) (*Atlas, error) {
	face, ok := p.faces[fontID]
	if !ok {
		return nil, errors.New("unknown font")
	}
	return NewAtlasForFace(fontID, px, face, []rune(text))
}

func newMultiFontTypefaceProvider(t *testing.T) *multiFontTypefaceProvider {
	t.Helper()
	faceOne, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	faceTwo, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return &multiFontTypefaceProvider{faces: map[render.FontID]*typeface.Face{1: faceOne, 2: faceTwo}}
}

type testTypefaceProvider struct {
	builtin *Atlas
	face    *typeface.Face
}

type recordingProvider struct {
	testTypefaceProvider
	calls []string
}

type cachedRecordingProvider struct {
	atlas *Atlas
	calls []string
}

func newCachedRecordingProvider(t *testing.T) *cachedRecordingProvider {
	t.Helper()
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := NewAtlasForFace(1, 18, face, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &cachedRecordingProvider{atlas: atlas}
}

func (p *cachedRecordingProvider) CachedAtlas(fontID render.FontID, px int) (*Atlas, bool) {
	return p.atlas, fontID == p.atlas.Key.Font && px == p.atlas.Key.PX
}

func (p *cachedRecordingProvider) AtlasFor(_ render.FontID, _ int, text string) (*Atlas, error) {
	p.calls = append(p.calls, text)
	return p.atlas, nil
}

func (p *recordingProvider) AtlasFor(fontID render.FontID, px int, text string) (*Atlas, error) {
	p.calls = append(p.calls, text)
	return p.testTypefaceProvider.AtlasFor(fontID, px, text)
}

func newTestTypefaceProvider(t *testing.T) *testTypefaceProvider {
	t.Helper()
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return &testTypefaceProvider{builtin: NewAtlas(), face: face}
}

func (p *testTypefaceProvider) AtlasFor(fontID render.FontID, px int, text string) (*Atlas, error) {
	if fontID == 0 {
		return p.builtin, nil
	}
	if fontID != 1 {
		return nil, errors.New("unknown font")
	}
	return NewAtlasForFace(fontID, px, p.face, []rune(text))
}

type builtTextRun struct {
	StreamRun
	Text TextMesh
}

func buildTextRun(t *testing.T, provider AtlasProvider, text string, fontID render.FontID) builtTextRun {
	t.Helper()
	cmd := render.DrawText{
		Position: math2d.Vec2{X: 10, Y: 30}, Text: text,
		Color: render.RGBA(1, 1, 1, 1), Size: 18, Font: fontID,
	}
	var builder Builder
	runs := builder.Build(Frame{
		Camera:   testCamera(),
		Commands: []render.Command{cmd},
	}, 100, 100, provider, nil)
	if len(runs) != 1 || runs[0].Kind != DrawRunText {
		t.Fatalf("text produced runs %+v", runs)
	}
	atlas, err := provider.AtlasFor(runs[0].AtlasKey.Font, runs[0].AtlasKey.PX, text)
	if err != nil {
		t.Fatal(err)
	}
	var mesh TextMesh
	AppendText(&mesh, testCtx(), cmd, atlas)
	return builtTextRun{StreamRun: runs[0], Text: mesh}
}

func textCommand(layer int, fontID render.FontID, text string) render.DrawText {
	return render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Position:    math2d.Vec2{X: 10, Y: 30}, Text: text,
		Color: render.RGBA(1, 1, 1, 1), Size: 18, Font: fontID,
	}
}

func textMeshWidth(mesh TextMesh) float64 {
	minX, maxX := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, v := range mesh.Vertices {
		minX = min32(minX, v.X)
		maxX = max32(maxX, v.X)
	}
	return float64(maxX - minX)
}

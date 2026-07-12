package tess

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
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

func TestBuildTextRotationAndClipRemainFinite(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	runs := Build(runtime.Frame{
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
	if len(runs) != 1 || len(runs[0].Text.Vertices) == 0 {
		t.Fatalf("rotated clipped text produced %+v", runs)
	}
	for _, v := range runs[0].Text.Vertices {
		if math.IsNaN(float64(v.X)) || math.IsInf(float64(v.X), 0) ||
			math.IsNaN(float64(v.Y)) || math.IsInf(float64(v.Y), 0) {
			t.Fatalf("non-finite vertex %+v", v)
		}
	}
}

func TestBuildSplitsTextRunsByAtlasWithoutChangingPainterOrder(t *testing.T) {
	provider := newTestTypefaceProvider(t)
	runs := Build(runtime.Frame{
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
	runs := Build(runtime.Frame{
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

func TestBuildTwoExternalFontsProduceSeparateAtlasRuns(t *testing.T) {
	provider := newMultiFontTypefaceProvider(t)
	runs := Build(runtime.Frame{
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

func buildTextRun(t *testing.T, provider AtlasProvider, text string, fontID render.FontID) DrawRun {
	t.Helper()
	runs := Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{render.DrawText{
			Position: math2d.Vec2{X: 10, Y: 30}, Text: text,
			Color: render.RGBA(1, 1, 1, 1), Size: 18, Font: fontID,
		}},
	}, 100, 100, provider, nil)
	if len(runs) != 1 || runs[0].Kind != DrawRunText {
		t.Fatalf("text produced runs %+v", runs)
	}
	return runs[0]
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

package tess

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func customTextAtlas(font render.FontID, px int) *Atlas {
	glyph := Glyph{Advance: 9, Width: 8, Height: 10, OffsetX: 1, OffsetY: -9, index: 0}
	return &Atlas{
		Key:         AtlasKey{Font: font, PX: px},
		Glyphs:      map[rune]Glyph{'A': glyph, '?': glyph},
		asciiGlyphs: indexedASCIIGlyphs(map[rune]Glyph{'A': glyph, '?': glyph}),
		coverage:    map[rune]struct{}{'A': {}},
		metrics:     []textGlyphMetric{metricForGlyph(glyph)},
		Ascent:      9,
		Height:      10,
	}
}

func textBatchLabel(font render.FontID, text string) render.TextLabel {
	return render.TextLabel{
		Position: math2d.Vec2{X: 50, Y: 50}, Text: text, Size: 10, Font: font,
		Color: render.RGBA(1, 1, 1, 1),
	}
}

func TestPlanTextBatchBoundsMatchShaderCorners(t *testing.T) {
	custom := customTextAtlas(1, 10)
	plan := PlanTextBatch(testCamera(), 100, 100, []render.TextLabel{textBatchLabel(1, "A")}, func(render.FontID, int) *Atlas {
		return custom
	})

	if len(plan.Ranges) != 1 || plan.Ranges[0].Key != custom.Key || plan.Ranges[0].Start != 0 || plan.Ranges[0].Count != 1 {
		t.Fatalf("ranges = %+v, want one range covering the single glyph", plan.Ranges)
	}
	if !plan.HasBounds {
		t.Fatal("HasBounds = false, want true for one visible glyph")
	}
	want := math2d.Rect{Min: math2d.Vec2{X: 51, Y: 41}, Max: math2d.Vec2{X: 59, Y: 51}}
	if plan.Bounds != want {
		t.Fatalf("bounds = %+v, want %+v", plan.Bounds, want)
	}
	if len(plan.Glyphs) != TextGlyphStride || len(plan.Labels) != TextLabelStride {
		t.Fatalf("glyphs = %d bytes labels = %d bytes, want one record each", len(plan.Glyphs), len(plan.Labels))
	}
}

func TestPlanTextBatchGroupsOnlyConsecutiveSameAtlas(t *testing.T) {
	builtin := NewAtlas()
	custom := customTextAtlas(1, 10)
	resolve := func(font render.FontID, px int) *Atlas {
		if font == 0 {
			return builtin
		}
		return custom
	}
	labels := []render.TextLabel{
		textBatchLabel(0, "A"),
		textBatchLabel(1, "A"),
		textBatchLabel(0, "A"),
	}
	plan := PlanTextBatch(testCamera(), 100, 100, labels, resolve)

	want := []TextBatchRange{
		{Key: builtin.Key, Start: 0, Count: 1},
		{Key: custom.Key, Start: 1, Count: 1},
		{Key: builtin.Key, Start: 2, Count: 1},
	}
	if len(plan.Ranges) != len(want) {
		t.Fatalf("ranges = %+v, want %+v (builtin/custom/builtin never merge across a different atlas)", plan.Ranges, want)
	}
	for i, w := range want {
		if plan.Ranges[i] != w {
			t.Fatalf("range %d = %+v, want %+v", i, plan.Ranges[i], w)
		}
	}
	if len(plan.Atlases) != 2 || plan.Atlases[builtin.Key] != builtin || plan.Atlases[custom.Key] != custom {
		t.Fatalf("atlases = %+v, want both keys resolved to their exact pointers", plan.Atlases)
	}
}

func TestPlanTextBatchSkippedLabelMergesSurroundingRange(t *testing.T) {
	builtin := NewAtlas()
	resolve := func(font render.FontID, px int) *Atlas {
		if font == 0 {
			return builtin
		}
		return nil // unresolvable font: the label contributes no records
	}
	labels := []render.TextLabel{
		textBatchLabel(0, "A"),
		textBatchLabel(2, "A"), // resolve fails, skipped entirely
		textBatchLabel(0, "A"),
	}
	plan := PlanTextBatch(testCamera(), 100, 100, labels, resolve)

	if len(plan.Ranges) != 1 || plan.Ranges[0].Count != 2 {
		t.Fatalf("ranges = %+v, want one merged range of 2 glyphs (the skipped label leaves no gap)", plan.Ranges)
	}
}

func TestPlanTextBatchEmptyInputs(t *testing.T) {
	if plan := PlanTextBatch(testCamera(), 100, 100, nil, func(render.FontID, int) *Atlas { return NewAtlas() }); plan.HasBounds || len(plan.Ranges) != 0 {
		t.Fatalf("empty labels plan = %+v, want zero content", plan)
	}
	if plan := PlanTextBatch(testCamera(), 100, 100, []render.TextLabel{textBatchLabel(0, "A")}, nil); plan.HasBounds || len(plan.Ranges) != 0 {
		t.Fatalf("nil resolve plan = %+v, want zero content", plan)
	}
}

func TestPlanTextBatchDegeneratePixelSizeSkipsLabel(t *testing.T) {
	degenerate := render.Camera{} // zero World/Viewport: WorldScalarToScreen returns 0
	plan := PlanTextBatch(degenerate, 0, 0, []render.TextLabel{textBatchLabel(0, "A")}, func(render.FontID, int) *Atlas { return NewAtlas() })
	if plan.HasBounds || len(plan.Ranges) != 0 {
		t.Fatalf("degenerate-camera plan = %+v, want zero content (px <= 0 skips the label)", plan)
	}
}

func TestTextBatchCoverageAggregatesRunesPerKey(t *testing.T) {
	labels := []render.TextLabel{
		textBatchLabel(0, "ça"), // font 0 included: the embedded default font grows dynamically
		textBatchLabel(1, "AB"),
		textBatchLabel(1, "BC"),
		textBatchLabel(2, "Z"),
	}
	coverage := TextBatchCoverage(testCamera(), 100, 100, labels)

	if len(coverage) != 3 {
		t.Fatalf("coverage keys = %d, want 3 (fonts 0, 1 and 2)", len(coverage))
	}
	key0 := AtlasKey{Font: 0, PX: 10}
	set0, ok := coverage[key0]
	if !ok || len(set0) != 2 {
		t.Fatalf("coverage[%+v] = %v, want the union {ç,a}", key0, set0)
	}
	for _, r := range []rune{'ç', 'a'} {
		if _, ok := set0[r]; !ok {
			t.Fatalf("coverage[%+v] missing rune %q", key0, r)
		}
	}
	key1 := AtlasKey{Font: 1, PX: 10}
	set1, ok := coverage[key1]
	if !ok || len(set1) != 3 {
		t.Fatalf("coverage[%+v] = %v, want the union {A,B,C}", key1, set1)
	}
	for _, r := range []rune{'A', 'B', 'C'} {
		if _, ok := set1[r]; !ok {
			t.Fatalf("coverage[%+v] missing rune %q", key1, r)
		}
	}
	key2 := AtlasKey{Font: 2, PX: 10}
	if set2, ok := coverage[key2]; !ok || len(set2) != 1 {
		t.Fatalf("coverage[%+v] = %v, want {Z}", key2, set2)
	}
}

func TestTextBatchCoverageEmptyWhenTextless(t *testing.T) {
	labels := []render.TextLabel{{Font: 0, Text: "", Size: 10}, {Font: 1, Text: "", Size: 10}}
	if coverage := TextBatchCoverage(testCamera(), 100, 100, labels); coverage != nil {
		t.Fatalf("coverage = %v, want nil", coverage)
	}
}

func TestTextPixelSizeCapsOversizedAtlases(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10_000, Y: 10_000}),
		Aspect:   render.AspectFit,
	}
	coverage := TextBatchCoverage(camera, 10_000, 10_000, []render.TextLabel{
		{Font: 1, Text: "large", Size: 10},
	})
	key := AtlasKey{Font: 1, PX: maxAtlasPixelSize}
	if _, ok := coverage[key]; !ok {
		t.Fatalf("coverage keys = %v, want capped key %+v", coverage, key)
	}
}

package tess

import (
	"strconv"
	"testing"

	"github.com/dhannyell/arasy/render"
)

// makeTextBatchLabels generates short HUD-style labels spread over a
// browser-scale world, cycling through benchTextSizes so PlanTextBatch's
// (font, px) grouping sees several distinct keys, like a realistic HUD/crowd
// name-tag batch.
func makeTextBatchLabels(n int, font render.FontID) []render.TextLabel {
	labels := make([]render.TextLabel, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		labels[i] = render.TextLabel{
			Position: pos,
			Text:     "Score: " + strconv.Itoa(i%10000),
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     benchTextSizes[i%len(benchTextSizes)],
			Font:     font,
		}
	}
	return labels
}

// BenchmarkPlanTextBatchReplace measures the full retained-batch repack cost
// (grouping, appendTextGlyphRecords, bounds) a replace or a
// projection/atlas invalidation pays, using the built-in font atlas. Steady
// state never reaches this path — finalizeTextBatches only calls it when
// content, projection, or atlas identity changed.
func BenchmarkPlanTextBatchReplace(b *testing.B) {
	cam := benchCamera()
	builtin := NewAtlas()
	resolve := func(render.FontID, int) *Atlas { return builtin }

	for _, n := range []int{100, 1000, 4000} {
		labels := makeTextBatchLabels(n, 0)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = PlanTextBatch(cam, 1920, 1080, labels, resolve)
			}
		})
	}
}

// BenchmarkPlanTextBatchExternalFont measures the same repack cost against a
// proportional TTF atlas instead of the built-in bitmap font, matching the
// browser's steady-state font path.
func BenchmarkPlanTextBatchExternalFont(b *testing.B) {
	cam := benchCamera()
	provider := newBenchmarkTypefaceProvider(b)

	for _, n := range []int{100, 1000, 4000} {
		labels := makeTextBatchLabels(n, 1)
		var runes []rune
		for _, l := range labels {
			runes = append(runes, []rune(l.Text)...)
		}
		for _, size := range benchTextSizes {
			px := int(cam.WorldScalarToScreen(size))
			if _, err := provider.AtlasFor(1, px, string(runes)); err != nil {
				b.Fatalf("warm atlas at px=%d: %v", px, err)
			}
		}
		resolve := func(font render.FontID, px int) *Atlas {
			atlas, _ := provider.AtlasFor(font, px, "")
			return atlas
		}
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = PlanTextBatch(cam, 1920, 1080, labels, resolve)
			}
		})
	}
}

// BenchmarkTextBatchCoverage measures the pre-Build coverage-aggregation
// pass a dirty or projection-invalidated batch pays: one rune set per
// (FontID, pixel size) key, built from every label's text.
func BenchmarkTextBatchCoverage(b *testing.B) {
	cam := benchCamera()

	for _, n := range []int{100, 1000, 4000} {
		labels := makeTextBatchLabels(n, 1)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = TextBatchCoverage(cam, 1920, 1080, labels)
			}
		})
	}
}

// BenchmarkPlanTextBatchSteadyStateSkip is a control: labels resolved
// against a nil-returning font (as if the caller already knows nothing
// changed) still walk every label to recompute px and skip it. This bounds
// the floor cost finalizeTextBatches' non-repack path would pay if it ever
// called PlanTextBatch speculatively — which it does not; see
// finalizeTextBatches in platform/wasm/webgpu_textbatch.go.
func BenchmarkPlanTextBatchSteadyStateSkip(b *testing.B) {
	cam := benchCamera()
	labels := makeTextBatchLabels(4000, 1)
	resolve := func(render.FontID, int) *Atlas { return nil }

	b.ReportAllocs()
	for b.Loop() {
		_ = PlanTextBatch(cam, 1920, 1080, labels, resolve)
	}
}

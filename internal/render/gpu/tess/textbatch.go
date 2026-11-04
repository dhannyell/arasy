package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// TextBatchRange is one consecutive span of a retained text batch's glyph
// records sharing one atlas. Start and Count are glyph-record indices,
// matching the streaming DrawRunText draw convention (firstVertex =
// offset*6, vertexCount = count*6). A Slug range has no Key, and its indices
// are records of TextBatchPlan.SlugInstances.
type TextBatchRange struct {
	Slug  bool
	Key   AtlasKey
	Start int
	Count int
}

// TextBatchPlan is the CPU-side result of packing one retained text batch:
// the glyph and label records to upload once, consecutive draw ranges
// grouped by the atlas each range actually resolved to, the atlas used by
// every active key (for staleness tracking across frames), and the
// screen-space bounds of every visible glyph quad for whole-batch culling.
type TextBatchPlan struct {
	Glyphs []byte
	Labels []byte
	Ranges []TextBatchRange
	// SlugInstances holds the records of the Slug ranges.
	SlugInstances []byte
	Atlases       map[AtlasKey]*Atlas
	Bounds        math2d.Rect
	HasBounds     bool
}

// textPixelSize resolves the screen pixel size PlanTextBatch and
// TextBatchCoverage both key atlas lookups by, mirroring VisitText's
// streaming default: a non-positive Size falls back to the built-in atlas's
// native glyph height before camera scaling.
func textPixelSize(c ctx, size float64) int {
	if size <= 0 {
		size = glyphHeight
	}
	return min(int(math.Round(c.cam.WorldScalarToScreen(size))), maxAtlasPixelSize)
}

// TextBatchCoverage aggregates, for every label, the (FontID, pixel size)
// key and the rune set it needs, at the pixel size PlanTextBatch will
// request. Font 0 labels are included: when the embedded default font is
// compiled in, its atlas grows dynamically like any user font. Providers
// serving the fixed built-in bitmap atlas simply ignore the growth request.
// Callers request coverage for these keys before PlanTextBatch, so a repack
// never grows atlas coverage itself. Labels that slug draws need no coverage.
func TextBatchCoverage(slug SlugText, cam render.Camera, w, h int, labels []render.TextLabel) map[AtlasKey]map[rune]struct{} {
	c := ctx{cam: cam, w: float64(w), h: float64(h)}
	var coverage map[AtlasKey]map[rune]struct{}
	for i := range labels {
		label := &labels[i]
		if label.Text == "" || drawsSlug(slug, label.Strategy, label.Font, label.Size) {
			continue
		}
		px := textPixelSize(c, label.Size)
		if px <= 0 {
			continue
		}
		key := AtlasKey{Font: label.Font, PX: px}
		if coverage == nil {
			coverage = make(map[AtlasKey]map[rune]struct{})
		}
		set := coverage[key]
		if set == nil {
			set = make(map[rune]struct{})
			coverage[key] = set
		}
		for _, r := range label.Text {
			set[r] = struct{}{}
		}
	}
	return coverage
}

// PlanTextBatch packs labels into glyph and label records without
// reordering, grouping only consecutive labels that resolve to the same
// atlas into one TextBatchRange. resolve must return an already-finalized
// atlas for (font, px) — including any fallback to the built-in font — with
// no coverage growth; the caller is responsible for requesting coverage
// (TextBatchCoverage) before calling PlanTextBatch. A nil result from
// resolve skips the label, like an unresolvable atlas skips a streaming
// DrawText. slug draws the labels with the GlyphSlug strategy, and a nil slug
// draws them with an atlas. A Slug range follows the same painter order.
func PlanTextBatch(slug SlugText, cam render.Camera, w, h int, labels []render.TextLabel, resolve func(font render.FontID, px int) *Atlas) TextBatchPlan {
	plan := TextBatchPlan{}
	if len(labels) == 0 || (resolve == nil && slug == nil) {
		return plan
	}

	c := ctx{cam: cam, w: float64(w), h: float64(h)}
	var glyphs []textGlyphRecord
	var labelRecs []textLabelRecord
	lastKey := AtlasKey{}
	lastSlug := false
	hasLast := false

	for i := range labels {
		label := &labels[i]
		px := textPixelSize(c, label.Size)
		if px <= 0 {
			continue
		}

		cmd := render.DrawText{
			Position: label.Position,
			Text:     label.Text,
			Color:    label.Color,
			Size:     label.Size,
			Font:     label.Font,
			Align:    label.Align,
			Baseline: label.Baseline,
			Rotation: label.Rotation,
		}
		if drawsSlug(slug, label.Strategy, label.Font, label.Size) {
			start := len(plan.SlugInstances) / SlugInstanceStride
			var bounds math2d.Rect
			plan.SlugInstances, bounds = slug.AppendInstances(plan.SlugInstances, cmd, c.cam, unclippedMin, unclippedMax)
			added := len(plan.SlugInstances)/SlugInstanceStride - start
			if added == 0 {
				continue
			}
			if hasLast && lastSlug {
				plan.Ranges[len(plan.Ranges)-1].Count += added
			} else {
				plan.Ranges = append(plan.Ranges, TextBatchRange{Slug: true, Start: start, Count: added})
				lastSlug, hasLast = true, true
			}
			expandRect(&plan.Bounds, &plan.HasBounds, bounds)
			continue
		}
		if resolve == nil {
			continue
		}
		atlas := resolve(label.Font, px)
		if atlas == nil {
			continue
		}
		start := len(glyphs)
		added := appendTextGlyphRecords(&glyphs, &labelRecs, c, cmd, atlas)
		if added == 0 {
			continue
		}

		rkey := atlas.Key
		if plan.Atlases == nil {
			plan.Atlases = make(map[AtlasKey]*Atlas)
		}
		plan.Atlases[rkey] = atlas

		if hasLast && !lastSlug && lastKey == rkey {
			plan.Ranges[len(plan.Ranges)-1].Count += added
		} else {
			plan.Ranges = append(plan.Ranges, TextBatchRange{Key: rkey, Start: start, Count: added})
			lastKey, lastSlug, hasLast = rkey, false, true
		}

		lr := labelRecs[len(labelRecs)-1]
		for _, g := range glyphs[start:] {
			expandTextBatchBounds(&plan.Bounds, &plan.HasBounds, g, lr, atlas.metrics[g.metric])
		}
	}

	plan.Glyphs = textGlyphBytes(glyphs)
	plan.Labels = textLabelBytes(labelRecs)
	return plan
}

// The GPU scissor applies the clip of a batch, and a parallax offset moves the
// glyphs after the instance clip test, so batch instances have no clip.
var (
	unclippedMin = math2d.Vec2{X: -math.MaxFloat32, Y: -math.MaxFloat32}
	unclippedMax = math2d.Vec2{X: math.MaxFloat32, Y: math.MaxFloat32}
)

// drawsSlug reports if the text goes to slug instead of a glyph atlas.
func drawsSlug(slug SlugText, strategy render.GlyphStrategy, font render.FontID, size float64) bool {
	return strategy == render.GlyphSlug && slug != nil && size > 0 && slug.Has(font)
}

func expandRect(bounds *math2d.Rect, has *bool, r math2d.Rect) {
	if !*has {
		*bounds, *has = r, true
		return
	}
	bounds.Min.X = min(bounds.Min.X, r.Min.X)
	bounds.Min.Y = min(bounds.Min.Y, r.Min.Y)
	bounds.Max.X = max(bounds.Max.X, r.Max.X)
	bounds.Max.Y = max(bounds.Max.Y, r.Max.Y)
}

// expandTextBatchBounds grows bounds to cover one glyph quad's screen-space
// corners, reproducing shaders/text_instanced.wgsl's vs_main exactly (same
// center/half-size/rotation formula) so culling never diverges from what the
// shader actually draws.
func expandTextBatchBounds(bounds *math2d.Rect, has *bool, g textGlyphRecord, l textLabelRecord, m textGlyphMetric) {
	halfW := m.width * l.scale * 0.5
	halfH := m.height * l.scale * 0.5
	centerX := l.originX + g.penX + (m.offsetX+m.width*0.5)*l.scale
	centerY := l.baselineY + (m.offsetY+m.height*0.5)*l.scale

	corners := [4][2]float32{
		{centerX - halfW, centerY - halfH},
		{centerX + halfW, centerY - halfH},
		{centerX + halfW, centerY + halfH},
		{centerX - halfW, centerY + halfH},
	}
	for _, corner := range corners {
		rx := corner[0]*l.transformA + corner[1]*l.transformC
		ry := corner[0]*l.transformB + corner[1]*l.transformD
		p := math2d.Vec2{X: float64(l.positionX + rx), Y: float64(l.positionY + ry)}
		if !*has {
			bounds.Min, bounds.Max = p, p
			*has = true
			continue
		}
		bounds.Min.X = min(bounds.Min.X, p.X)
		bounds.Min.Y = min(bounds.Min.Y, p.Y)
		bounds.Max.X = max(bounds.Max.X, p.X)
		bounds.Max.Y = max(bounds.Max.Y, p.Y)
	}
}

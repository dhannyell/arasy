package tess

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	atlasFirstRune = 32
	atlasLastRune  = 126
	atlasCols      = 16
	glyphWidth     = 7
	glyphHeight    = 13
	atlasPadding   = 1
)

// AtlasKey identifies one rasterized font atlas for a concrete screen
// pixel size. Font 0 is the built-in engine bitmap font.
type AtlasKey struct {
	Font render.FontID
	PX   int
}

// Glyph stores one atlas entry: UVs plus pixel-space advance and bounds.
type Glyph struct {
	UV      [4]float32
	Advance float64
	Width   float64
	Height  float64
	OffsetX float64
	OffsetY float64
	index   uint32
}

// Atlas stores the bitmap font texture and per-rune UV coordinates.
type Atlas struct {
	Key   AtlasKey
	Image *image.Alpha
	// LinearFilter is true for rasterized TTF/OpenType atlases. The bitmap
	// fallback deliberately keeps nearest-neighbor sampling so its pixels stay
	// crisp.
	LinearFilter bool
	Glyphs       map[rune]Glyph
	// asciiGlyphs bypasses map lookups for printable ASCII, which every atlas
	// owns. The map remains the source of truth for dynamic Unicode coverage.
	asciiGlyphs [128]Glyph
	// coverage includes requested runes that the face could not rasterize. It
	// prevents a provider from rebuilding an unchanged atlas every frame just
	// because layout falls back to '?'.
	coverage map[rune]struct{}
	metrics  []textGlyphMetric
	Ascent   float64
	Height   float64
}

// AtlasProvider resolves an atlas for one command. Implementations may cache
// by (font, px), extending the atlas when text contains previously unseen
// runes. Returning nil or an error makes Build fall back to the built-in font.
type AtlasProvider interface {
	AtlasFor(font render.FontID, px int, text string) (*Atlas, error)
}

// CachedAtlasProvider is an optional steady-state fast path. Build still
// checks the cached atlas's rune coverage and calls AtlasFor when new Unicode
// appears, so providers retain ownership of atlas growth.
type CachedAtlasProvider interface {
	CachedAtlas(font render.FontID, px int) (*Atlas, bool)
}

// AtlasFor lets the legacy built-in atlas act as a provider. External font
// requests deliberately fail so Build can use the readable built-in fallback.
func (a *Atlas) AtlasFor(fontID render.FontID, _ int, _ string) (*Atlas, error) {
	if a == nil {
		return nil, errors.New("nil font atlas")
	}
	if fontID != 0 {
		return nil, errors.New("font is not available in the built-in atlas")
	}
	return a, nil
}

// TextVertex is one interleaved text vertex: clip-space position, straight
// alpha color, and atlas UV coordinates.
type TextVertex struct {
	X, Y       float32
	R, G, B, A float32
	U, V       float32
}

// TextMesh accumulates triangles for one text atlas.
type TextMesh struct {
	Vertices []TextVertex
	// glyphScratch is AppendText's per-command glyph lookup buffer, reused
	// across commands so steady-state tessellation does not allocate.
	glyphScratch []Glyph
}

func (m *TextMesh) append(vertices ...TextVertex) {
	m.Vertices = append(m.Vertices, vertices...)
}

// NewAtlas rasterizes the built-in monospace bitmap font into a single alpha
// texture and records UVs for printable ASCII runes.
func NewAtlas() *Atlas {
	rows := (atlasLastRune - atlasFirstRune + atlasCols) / atlasCols
	img := image.NewAlpha(image.Rect(0, 0, atlasCols*glyphWidth, rows*glyphHeight))
	glyphs := make(map[rune]Glyph, atlasLastRune-atlasFirstRune+1)

	face := basicfont.Face7x13
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Alpha{A: 255}),
		Face: face,
	}
	metrics := face.Metrics()

	for r := rune(atlasFirstRune); r <= rune(atlasLastRune); r++ {
		i := int(r) - atlasFirstRune
		col := i % atlasCols
		row := i / atlasCols
		x := col * glyphWidth
		y := row * glyphHeight

		drawer.Dot = fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y) + metrics.Ascent}
		drawer.DrawString(string(r))

		glyphs[r] = Glyph{
			UV: [4]float32{
				float32(x) / float32(img.Bounds().Dx()),
				float32(y) / float32(img.Bounds().Dy()),
				float32(x+glyphWidth) / float32(img.Bounds().Dx()),
				float32(y+glyphHeight) / float32(img.Bounds().Dy()),
			},
			Advance: float64(glyphWidth),
			Width:   float64(glyphWidth),
			Height:  float64(glyphHeight),
			OffsetY: -fixedFloat(metrics.Ascent),
			index:   uint32(i),
		}
		// NewAtlas owns printable ASCII, so this indexed copy is safe for the
		// built-in-only fast path in appendTextGlyphRecords.
		// The map remains populated for the legacy tessellation oracle.
	}
	asciiGlyphs := indexedASCIIGlyphs(glyphs)
	glyphMetrics := make([]textGlyphMetric, 0, len(glyphs))
	for r := rune(atlasFirstRune); r <= rune(atlasLastRune); r++ {
		glyph := glyphs[r]
		metric := metricForGlyph(glyph)
		// Canonical FontID(0) layout uses render.MeasureText's advance,
		// not the bitmap cell width. The legacy tessellator stretches each
		// cell to that advance; preserve the same geometry in the GPU table.
		measured := render.MeasureText("M", glyphHeight)
		metric.width = float32(measured.X * float64(glyphHeight) / measured.Y)
		glyphMetrics = append(glyphMetrics, metric)
	}

	return &Atlas{
		Key:          AtlasKey{Font: 0, PX: glyphHeight},
		Image:        img,
		LinearFilter: false,
		Glyphs:       glyphs,
		asciiGlyphs:  asciiGlyphs,
		coverage:     runeSet(atlasFirstRune, atlasLastRune, nil),
		metrics:      glyphMetrics,
		Ascent:       fixedFloat(metrics.Ascent),
		Height:       fixedFloat(metrics.Height),
	}
}

type packedGlyph struct {
	r       rune
	bounds  fixed.Rectangle26_6
	advance fixed.Int26_6
	x, y    int
	w, h    int
}

// NewAtlasForFace rasterizes printable ASCII plus the requested runes using
// real glyph metrics. Glyphs are packed in shelves, and '?' is always present
// so unsupported Unicode remains visible without growing an unbounded atlas.
func NewAtlasForFace(fontID render.FontID, px int, parsed *typeface.Face, runes []rune) (*Atlas, error) {
	if parsed == nil || parsed.Font() == nil {
		return nil, errors.New("create font atlas: nil typeface")
	}
	if px <= 0 {
		return nil, errors.New("create font atlas: pixel size must be positive")
	}

	fontFace, err := opentype.NewFace(parsed.Font(), &opentype.FaceOptions{
		Size: float64(px), DPI: 72, Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, err
	}

	requested := make(map[rune]struct{}, atlasLastRune-atlasFirstRune+1+len(runes))
	for r := rune(atlasFirstRune); r <= rune(atlasLastRune); r++ {
		requested[r] = struct{}{}
	}
	requested['?'] = struct{}{}
	for _, r := range runes {
		requested[r] = struct{}{}
	}
	ordered := make([]rune, 0, len(requested))
	for r := range requested {
		ordered = append(ordered, r)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	items := make([]packedGlyph, 0, len(ordered))
	totalArea := 0
	maxCellWidth := 1
	for _, r := range ordered {
		bounds, advance, ok := fontFace.GlyphBounds(r)
		if !ok {
			// Do not give an unsupported rune a misleading UV. AppendText will
			// resolve its absent entry to the atlas's guaranteed '?' glyph.
			continue
		}
		w := max(1, bounds.Max.X.Ceil()-bounds.Min.X.Floor())
		h := max(1, bounds.Max.Y.Ceil()-bounds.Min.Y.Floor())
		cellW := w + 2*atlasPadding
		cellH := h + 2*atlasPadding
		maxCellWidth = max(maxCellWidth, cellW)
		totalArea += cellW * cellH
		items = append(items, packedGlyph{r: r, bounds: bounds, advance: advance, w: w, h: h})
	}

	// A square-ish power-of-two shelf width keeps uploads compact while
	// remaining deterministic for a given rune set.
	targetWidth := max(maxCellWidth, nextPowerOfTwo(int(math.Ceil(math.Sqrt(float64(totalArea))))))
	x, y, rowHeight := 0, 0, 0
	for i := range items {
		cellW := items[i].w + 2*atlasPadding
		cellH := items[i].h + 2*atlasPadding
		if x > 0 && x+cellW > targetWidth {
			x = 0
			y += rowHeight
			rowHeight = 0
		}
		items[i].x = x + atlasPadding
		items[i].y = y + atlasPadding
		x += cellW
		rowHeight = max(rowHeight, cellH)
	}
	atlasHeight := max(1, y+rowHeight)
	img := image.NewAlpha(image.Rect(0, 0, targetWidth, atlasHeight))
	drawer := &font.Drawer{Dst: img, Src: image.NewUniform(color.Alpha{A: 255}), Face: fontFace}
	glyphs := make(map[rune]Glyph, len(items))

	for index, item := range items {
		drawer.Dot = fixed.Point26_6{
			X: fixed.I(item.x) - item.bounds.Min.X,
			Y: fixed.I(item.y) - item.bounds.Min.Y,
		}
		drawer.DrawString(string(item.r))
		glyphs[item.r] = Glyph{
			UV: [4]float32{
				float32(item.x) / float32(targetWidth),
				float32(item.y) / float32(atlasHeight),
				float32(item.x+item.w) / float32(targetWidth),
				float32(item.y+item.h) / float32(atlasHeight),
			},
			Advance: fixedFloat(item.advance),
			Width:   float64(item.w),
			Height:  float64(item.h),
			OffsetX: fixedFloat(item.bounds.Min.X),
			OffsetY: fixedFloat(item.bounds.Min.Y),
			index:   uint32(index),
		}
	}

	metrics := fontFace.Metrics()
	asciiGlyphs := indexedASCIIGlyphs(glyphs)
	glyphMetrics := make([]textGlyphMetric, len(items))
	for _, glyph := range glyphs {
		glyphMetrics[glyph.index] = metricForGlyph(glyph)
	}
	atlas := &Atlas{
		Key:          AtlasKey{Font: fontID, PX: px},
		Image:        img,
		LinearFilter: true,
		Glyphs:       glyphs,
		asciiGlyphs:  asciiGlyphs,
		coverage:     requested,
		metrics:      glyphMetrics,
		Ascent:       fixedFloat(metrics.Ascent),
		Height:       fixedFloat(metrics.Height),
	}
	if err := fontFace.Close(); err != nil {
		return nil, fmt.Errorf("create font atlas: close font face: %w", err)
	}
	return atlas, nil
}

// NewDefaultFaceAtlas is like NewAtlasForFace but substitutes
// render.MeasureText advances for every glyph, preserving font-0's canonical
// deterministic layout regardless of the default face's real metrics. The
// glyph metrics retain each glyph's visual bounds so narrow glyphs are not
// stretched to fill the monospace advance.
func NewDefaultFaceAtlas(px int, parsed *typeface.Face, runes []rune) (*Atlas, error) {
	atlas, err := NewAtlasForFace(0, px, parsed, runes)
	if err != nil {
		return nil, err
	}
	measured := render.MeasureText("M", float64(px))
	adv := float32(measured.X * float64(px) / measured.Y)
	for r, g := range atlas.Glyphs {
		g.Advance = float64(adv)
		atlas.Glyphs[r] = g
	}
	for r := rune(atlasFirstRune); r <= rune(atlasLastRune); r++ {
		g := atlas.asciiGlyphs[r]
		g.Advance = float64(adv)
		atlas.asciiGlyphs[r] = g
	}
	return atlas, nil
}

// GlyphMetricStream returns immutable atlas metadata in the storage-buffer
// layout consumed by the instanced text shader.
func (a *Atlas) GlyphMetricStream() []byte {
	if a == nil {
		return nil
	}
	return textMetricBytes(a.metrics)
}

func metricForGlyph(glyph Glyph) textGlyphMetric {
	return textGlyphMetric{
		uvMinX: glyph.UV[0], uvMinY: glyph.UV[1], uvMaxX: glyph.UV[2], uvMaxY: glyph.UV[3],
		width: float32(glyph.Width), height: float32(glyph.Height),
		offsetX: float32(glyph.OffsetX), offsetY: float32(glyph.OffsetY),
	}
}

// indexedASCIIGlyphs keeps printable-ASCII lookup branchless while matching
// the map-based layout fallback for faces that omit a printable glyph.
func indexedASCIIGlyphs(glyphs map[rune]Glyph) [128]Glyph {
	var indexed [128]Glyph
	fallback := glyphs['?']
	for r := rune(atlasFirstRune); r <= rune(atlasLastRune); r++ {
		glyph, ok := glyphs[r]
		if !ok {
			glyph = fallback
		}
		indexed[r] = glyph
	}
	return indexed
}

func runeSet(first, last rune, extra []rune) map[rune]struct{} {
	set := make(map[rune]struct{}, int(last-first+1)+len(extra))
	for r := first; r <= last; r++ {
		set[r] = struct{}{}
	}
	for _, r := range extra {
		set[r] = struct{}{}
	}
	return set
}

func (a *Atlas) covers(r rune) bool {
	if a == nil {
		return false
	}
	if r >= atlasFirstRune && r <= atlasLastRune {
		return true
	}
	_, ok := a.coverage[r]
	return ok
}

func printableASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < atlasFirstRune || text[i] > atlasLastRune {
			return false
		}
	}
	return true
}

func (a *Atlas) glyph(r rune) Glyph {
	if r >= atlasFirstRune && r <= atlasLastRune {
		return a.asciiGlyphs[r]
	}
	if glyph, ok := a.Glyphs[r]; ok {
		return glyph
	}
	return a.asciiGlyphs['?']
}

func fixedFloat(v fixed.Int26_6) float64 { return float64(v) / 64 }

func nextPowerOfTwo(v int) int {
	p := 1
	for p < v {
		p <<= 1
	}
	return p
}

package tess

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/render"
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
}

// Atlas stores the bitmap font texture and per-rune UV coordinates.
type Atlas struct {
	Key    AtlasKey
	Image  *image.Alpha
	Glyphs map[rune]Glyph
	Ascent float64
	Height float64
}

// AtlasProvider resolves an atlas for one command. Implementations may cache
// by (font, px), extending the atlas when text contains previously unseen
// runes. Returning nil or an error makes Build fall back to the built-in font.
type AtlasProvider interface {
	AtlasFor(font render.FontID, px int, text string) (*Atlas, error)
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
		}
	}

	return &Atlas{
		Key:    AtlasKey{Font: 0, PX: glyphHeight},
		Image:  img,
		Glyphs: glyphs,
		Ascent: fixedFloat(metrics.Ascent),
		Height: fixedFloat(metrics.Height),
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

	for _, item := range items {
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
		}
	}

	metrics := fontFace.Metrics()
	atlas := &Atlas{
		Key:    AtlasKey{Font: fontID, PX: px},
		Image:  img,
		Glyphs: glyphs,
		Ascent: fixedFloat(metrics.Ascent),
		Height: fixedFloat(metrics.Height),
	}
	if err := fontFace.Close(); err != nil {
		return nil, fmt.Errorf("create font atlas: close font face: %w", err)
	}
	return atlas, nil
}

func fixedFloat(v fixed.Int26_6) float64 { return float64(v) / 64 }

func nextPowerOfTwo(v int) int {
	p := 1
	for p < v {
		p <<= 1
	}
	return p
}

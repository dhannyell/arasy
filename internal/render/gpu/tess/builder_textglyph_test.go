package tess

import (
	"encoding/binary"
	"math"
	"testing"
	"unsafe"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type decodedTextGlyph struct {
	penX          float32
	metric, label uint32
}

type decodedTextLabel struct {
	position, rotation [2]float32
	originBaseline     [2]float32
	scale              float32
	clipMin, clipMax   [2]float32
	tint               uint32
}

func decodeTextGlyph(t *testing.T, stream []byte, index int) decodedTextGlyph {
	t.Helper()
	offset := index * TextGlyphStride
	if len(stream) < offset+TextGlyphStride {
		t.Fatalf("glyph stream length = %d, cannot decode record %d", len(stream), index)
	}
	return decodedTextGlyph{
		penX:   math.Float32frombits(binary.LittleEndian.Uint32(stream[offset:])),
		metric: binary.LittleEndian.Uint32(stream[offset+4:]),
		label:  binary.LittleEndian.Uint32(stream[offset+8:]),
	}
}

func decodeTextLabel(t *testing.T, stream []byte, index int) decodedTextLabel {
	t.Helper()
	offset := index * TextLabelStride
	if len(stream) < offset+TextLabelStride {
		t.Fatalf("label stream length = %d, cannot decode record %d", len(stream), index)
	}
	vec := func(at int) [2]float32 {
		return [2]float32{
			math.Float32frombits(binary.LittleEndian.Uint32(stream[offset+at:])),
			math.Float32frombits(binary.LittleEndian.Uint32(stream[offset+at+4:])),
		}
	}
	return decodedTextLabel{
		position: vec(0), rotation: vec(8), originBaseline: vec(16),
		scale:   math.Float32frombits(binary.LittleEndian.Uint32(stream[offset+24:])),
		clipMin: vec(32), clipMax: vec(40),
		tint: binary.LittleEndian.Uint32(stream[offset+48:]),
	}
}

func TestBuilderTextGlyphRecordsMatchLegacyTextQuads(t *testing.T) {
	atlas := NewAtlas()
	cmd := render.DrawText{
		Position: math2d.Vec2{X: 45, Y: 40},
		Text:     "Hi",
		Color:    render.RGBA(0.25, 0.5, 0.75, 0.8),
		Size:     16,
		Align:    render.AlignCenter,
		Baseline: render.BaselineMiddle,
		Rotation: math.Pi / 5,
	}

	var builder Builder
	runs := builder.Build(Frame{Camera: testCamera(), Commands: []render.Command{cmd}}, 100, 100, atlas, nil)
	if len(runs) != 1 || runs[0].Kind != DrawRunText || runs[0].VertexCount != 2 {
		t.Fatalf("runs = %+v, want one two-glyph text run", runs)
	}
	stream := builder.TextGlyphStream()
	if len(stream) != 2*TextGlyphStride {
		t.Fatalf("glyph stream length = %d, want %d", len(stream), 2*TextGlyphStride)
	}
	labelStream := builder.TextLabelStream()
	if len(labelStream) != TextLabelStride {
		t.Fatalf("label stream length = %d, want %d", len(labelStream), TextLabelStride)
	}
	label := decodeTextLabel(t, labelStream, 0)
	if label.tint != uint32(0xccbf8040) {
		t.Fatalf("label tint = %#08x, want %#08x", label.tint, uint32(0xccbf8040))
	}
	if label.clipMin != [2]float32{} || label.clipMax != [2]float32{100, 100} {
		t.Fatalf("label clip = %v..%v, want [0 0]..[100 100]", label.clipMin, label.clipMax)
	}

	var legacy TextMesh
	ctx := testCtx()
	AppendText(&legacy, ctx, cmd, atlas)
	if len(legacy.Vertices) != 12 {
		t.Fatalf("legacy vertices = %d, want 12", len(legacy.Vertices))
	}
	for glyphIndex, r := range []rune(cmd.Text) {
		glyph := decodeTextGlyph(t, stream, glyphIndex)
		if glyph.label != 0 {
			t.Fatalf("glyph %d label = %d, want 0", glyphIndex, glyph.label)
		}
		metric := atlas.glyph(r)
		if glyph.metric != metric.index {
			t.Fatalf("glyph %d metric = %d, want %d", glyphIndex, glyph.metric, metric.index)
		}
		gpuMetric := atlas.metrics[glyph.metric]
		corners := [][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, -1}, {1, 1}, {-1, 1}}
		for cornerIndex, corner := range corners {
			halfX := gpuMetric.width * label.scale * 0.5
			halfY := gpuMetric.height * label.scale * 0.5
			localX := label.originBaseline[0] + glyph.penX + (gpuMetric.offsetX+gpuMetric.width*0.5)*label.scale + corner[0]*halfX
			localY := label.originBaseline[1] + (gpuMetric.offsetY+gpuMetric.height*0.5)*label.scale + corner[1]*halfY
			screen := math2d.Vec2{
				X: float64(label.position[0] + localX*label.rotation[0] - localY*label.rotation[1]),
				Y: float64(label.position[1] + localX*label.rotation[1] + localY*label.rotation[0]),
			}
			want := screenToClip(ctx, screen)
			got := legacy.Vertices[glyphIndex*6+cornerIndex]
			if math.Abs(float64(got.X-want[0])) > testEpsilon {
				t.Fatalf("glyph %d corner %d X got %v, want %v (local=%v,%v)", glyphIndex, cornerIndex, got.X, want[0], localX, localY)
			}
			assertFloat32(t, got.Y, want[1])
			if corner[0] < 0 {
				assertFloat32(t, got.U, gpuMetric.uvMinX)
			} else {
				assertFloat32(t, got.U, gpuMetric.uvMaxX)
			}
			if corner[1] < 0 {
				assertFloat32(t, got.V, gpuMetric.uvMinY)
			} else {
				assertFloat32(t, got.V, gpuMetric.uvMaxY)
			}
			assertFloat32(t, got.R, 0.25)
			assertFloat32(t, got.G, 0.5)
			assertFloat32(t, got.B, 0.75)
			assertFloat32(t, got.A, 0.8)
		}
	}
}

func TestTextGlyphRecordMatchesWGSLStride(t *testing.T) {
	if got := unsafe.Sizeof(textGlyphRecord{}); got != TextGlyphStride {
		t.Fatalf("text glyph record size = %d, want %d", got, TextGlyphStride)
	}
	if got := unsafe.Sizeof(textLabelRecord{}); got != TextLabelStride {
		t.Fatalf("text label record size = %d, want %d", got, TextLabelStride)
	}
	if got := unsafe.Sizeof(textGlyphMetric{}); got != TextMetricStride {
		t.Fatalf("text metric record size = %d, want %d", got, TextMetricStride)
	}
}

func TestTextGlyphRecordsUseQuestionMarkForUnsupportedASCII(t *testing.T) {
	fallback := Glyph{Advance: 6, Width: 5, Height: 8, index: 0}
	atlas := &Atlas{
		Key:         AtlasKey{Font: 1, PX: 10},
		Glyphs:      map[rune]Glyph{'?': fallback},
		asciiGlyphs: indexedASCIIGlyphs(map[rune]Glyph{'?': fallback}),
		metrics:     []textGlyphMetric{metricForGlyph(fallback)},
		Ascent:      8,
		Height:      10,
	}
	if got := atlas.glyph('B'); got != fallback {
		t.Fatalf("atlas.glyph('B') = %+v, want question-mark fallback %+v", got, fallback)
	}

	var glyphs []textGlyphRecord
	var labels []textLabelRecord
	count := appendTextGlyphRecords(&glyphs, &labels, testCtx(), render.DrawText{
		Position: math2d.Vec2{X: 10, Y: 20}, Text: "B",
		Color: render.RGBA(1, 1, 1, 1), Size: 10, Font: 1,
	}, atlas)
	if count != 1 || len(glyphs) != 1 || glyphs[0].metric != fallback.index {
		t.Fatalf("fallback glyph records = %+v count=%d, want one question-mark metric", glyphs, count)
	}
}

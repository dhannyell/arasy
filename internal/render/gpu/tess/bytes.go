package tess

import (
	"encoding/binary"
	"math"
	"unsafe"
)

const (
	SolidVertexStride = 24
	TextVertexStride  = 32
	TexVertexStride   = 40
	SpriteStride      = 64
	TextGlyphStride   = 16
	TextLabelStride   = 64
	TextMetricStride  = 32
)

// spriteRecord is the 64-byte storage layout consumed by both sprite WGSL
// shaders. All fields are 32-bit, so its in-memory representation is the
// little-endian byte layout WebGPU receives on every supported host.
type spriteRecord struct {
	centerX, centerY, halfWidth, halfHeight float32
	rotation                                float32
	flags                                   uint32
	uvMinX, uvMinY, uvMaxX, uvMaxY          float32
	clipMinX, clipMinY, clipMaxX, clipMaxY  float32
	tint, textureLayer                      uint32
}

// textGlyphRecord carries the per-glyph state consumed by
// shaders/text_instanced.wgsl. Label-wide transform, clip, and tint live in a
// separate textLabelRecord instead of being duplicated for every glyph.
type textGlyphRecord struct {
	penX               float32
	metric, label, pad uint32
}

// textLabelRecord carries state shared by all glyphs in one DrawText.
type textLabelRecord struct {
	positionX, positionY                   float32
	transformA, transformB                 float32
	transformC, transformD                 float32
	originX, baselineY                     float32
	clipMinX, clipMinY, clipMaxX, clipMaxY float32
	scale                                  float32
	tint, pad0, pad1                       uint32
}

// textGlyphMetric is immutable atlas metadata. Keeping it in a GPU buffer
// lets each per-frame glyph record contain only pen position and two indices.
type textGlyphMetric struct {
	uvMinX, uvMinY, uvMaxX, uvMaxY  float32
	width, height, offsetX, offsetY float32
}

func spriteBytes(records []spriteRecord) []byte {
	if len(records) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(records))), len(records)*SpriteStride)
}

func textGlyphBytes(records []textGlyphRecord) []byte {
	if len(records) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(records))), len(records)*TextGlyphStride)
}

func textLabelBytes(records []textLabelRecord) []byte {
	if len(records) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(records))), len(records)*TextLabelStride)
}

func textMetricBytes(records []textGlyphMetric) []byte {
	if len(records) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(records))), len(records)*TextMetricStride)
}

// Bytes serializes a solid-color mesh into the WebGPU vertex buffer layout.
//
// Each Vertex is encoded as six little-endian float32 values in field order:
// X, Y, R, G, B, A.
func Bytes(mesh Mesh) []byte {
	out := make([]byte, len(mesh.Vertices)*SolidVertexStride)
	for i, v := range mesh.Vertices {
		offset := i * SolidVertexStride
		putFloat32(out[offset:], v.X)
		putFloat32(out[offset+4:], v.Y)
		putFloat32(out[offset+8:], v.R)
		putFloat32(out[offset+12:], v.G)
		putFloat32(out[offset+16:], v.B)
		putFloat32(out[offset+20:], v.A)
	}
	return out
}

// TextBytes serializes a text mesh into the WebGPU vertex buffer layout.
//
// Each TextVertex is encoded as eight little-endian float32 values in field
// order: X, Y, R, G, B, A, U, V.
func TextBytes(mesh TextMesh) []byte {
	out := make([]byte, len(mesh.Vertices)*TextVertexStride)
	for i, v := range mesh.Vertices {
		offset := i * TextVertexStride
		putFloat32(out[offset:], v.X)
		putFloat32(out[offset+4:], v.Y)
		putFloat32(out[offset+8:], v.R)
		putFloat32(out[offset+12:], v.G)
		putFloat32(out[offset+16:], v.B)
		putFloat32(out[offset+20:], v.A)
		putFloat32(out[offset+24:], v.U)
		putFloat32(out[offset+28:], v.V)
	}
	return out
}

// TexBytes serializes a textured mesh into the WebGPU vertex buffer layout.
//
// Each TexVertex is encoded as ten little-endian float32 values in field
// order: XW, YW, 0 (a Z placeholder so position is a vec4 for the WGSL
// pipeline), W, R, G, B, A, U, V.
func TexBytes(mesh TexMesh) []byte {
	out := make([]byte, len(mesh.Vertices)*TexVertexStride)
	for i, v := range mesh.Vertices {
		offset := i * TexVertexStride
		putFloat32(out[offset:], v.XW)
		putFloat32(out[offset+4:], v.YW)
		putFloat32(out[offset+8:], 0)
		putFloat32(out[offset+12:], v.W)
		putFloat32(out[offset+16:], v.R)
		putFloat32(out[offset+20:], v.G)
		putFloat32(out[offset+24:], v.B)
		putFloat32(out[offset+28:], v.A)
		putFloat32(out[offset+32:], v.U)
		putFloat32(out[offset+36:], v.V)
	}
	return out
}

func putFloat32(dst []byte, v float32) {
	binary.LittleEndian.PutUint32(dst, math.Float32bits(v))
}

func appendSolidVertex(dst []byte, v Vertex) []byte {
	start := len(dst)
	dst = append(dst, make([]byte, SolidVertexStride)...)
	putFloat32(dst[start:], v.X)
	putFloat32(dst[start+4:], v.Y)
	putFloat32(dst[start+8:], v.R)
	putFloat32(dst[start+12:], v.G)
	putFloat32(dst[start+16:], v.B)
	putFloat32(dst[start+20:], v.A)
	return dst
}

func appendTexVertex(dst []byte, v TexVertex) []byte {
	start := len(dst)
	dst = append(dst, make([]byte, TexVertexStride)...)
	putFloat32(dst[start:], v.XW)
	putFloat32(dst[start+4:], v.YW)
	putFloat32(dst[start+8:], 0)
	putFloat32(dst[start+12:], v.W)
	putFloat32(dst[start+16:], v.R)
	putFloat32(dst[start+20:], v.G)
	putFloat32(dst[start+24:], v.B)
	putFloat32(dst[start+28:], v.A)
	putFloat32(dst[start+32:], v.U)
	putFloat32(dst[start+36:], v.V)
	return dst
}

// appendSprite writes one storage-buffer sprite record. Its field order and
// alignment match Sprite in shaders/sprite_render.wgsl.
func appendSprite(
	dst []spriteRecord,
	centerX, centerY, halfWidth, halfHeight, rotation float32,
	uvMin, uvMax, clipMin, clipMax [2]float32,
	tint, textureLayer uint32,
) []spriteRecord {
	return append(dst, spriteRecord{
		centerX: centerX, centerY: centerY, halfWidth: halfWidth, halfHeight: halfHeight,
		rotation: rotation,
		uvMinX:   uvMin[0], uvMinY: uvMin[1], uvMaxX: uvMax[0], uvMaxY: uvMax[1],
		clipMinX: clipMin[0], clipMinY: clipMin[1], clipMaxX: clipMax[0], clipMaxY: clipMax[1],
		tint: tint, textureLayer: textureLayer,
	})
}

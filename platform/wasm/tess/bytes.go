package tess

import (
	"encoding/binary"
	"math"
)

// Bytes serializes a solid-color mesh into the WebGPU vertex buffer layout.
//
// Each Vertex is encoded as six little-endian float32 values in field order:
// X, Y, R, G, B, A.
func Bytes(mesh Mesh) []byte {
	out := make([]byte, len(mesh.Vertices)*24)
	for i, v := range mesh.Vertices {
		offset := i * 24
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
	out := make([]byte, len(mesh.Vertices)*32)
	for i, v := range mesh.Vertices {
		offset := i * 32
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

func putFloat32(dst []byte, v float32) {
	binary.LittleEndian.PutUint32(dst, math.Float32bits(v))
}

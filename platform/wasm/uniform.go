package wasm

import (
	"encoding/binary"
	"math"
)

type particleUniformWriter struct {
	dst    []byte
	offset int
}

func (w *particleUniformWriter) f32(value float64) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], math.Float32bits(float32(value)))
	w.offset += 4
}

func (w *particleUniformWriter) u32(value uint32) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], value)
	w.offset += 4
}

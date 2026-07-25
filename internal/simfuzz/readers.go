package simfuzz

import (
	"encoding/binary"
	"hash/fnv"
	"math"
)

// readF64 reads a little-endian float64 at *offset, advancing it by 8. Missing
// bytes yield 0 and still advance the offset, so the layout stays stable.
func readF64(buf []byte, offset *int) float64 {
	if *offset+8 > len(buf) {
		*offset += 8
		return 0
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(buf[*offset:]))
	*offset += 8
	return v
}

// readF64Finite reads a float64 and forces it to a finite, bounded value. NaN
// and Inf become 0 (so P4 only ever flags NaN/Inf produced by the simulation,
// not injected by the decoder); finite values are clamped to [-maxFuzzFloat,
// maxFuzzFloat] to keep scenarios fast while still stressing the pipeline with
// extreme magnitudes.
const maxFuzzFloat = 1e6

func readF64Finite(buf []byte, offset *int) float64 {
	v := readF64(buf, offset)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v > maxFuzzFloat {
		return maxFuzzFloat
	}
	if v < -maxFuzzFloat {
		return -maxFuzzFloat
	}
	return v
}

func readU8(buf []byte, offset *int) uint8 {
	if *offset >= len(buf) {
		return 0
	}
	v := buf[*offset]
	*offset++
	return v
}

// Pack helpers write little-endian component data so a shaped decoder can build
// an EntityDef.Data that SpawnFromScript reads back exactly. They are the
// inverse of the read* helpers and stay in the same field order as compDataLen.
func packF64(v float64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	return b
}

func packI32(v int32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return b
}

// concatBytes joins the given byte slices into one, for assembling EntityDef.Data.
func concatBytes(parts ...[]byte) []byte {
	var n int
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func readU32(buf []byte, offset *int) uint32 {
	if *offset+4 > len(buf) {
		*offset += 4
		return 0
	}
	v := binary.LittleEndian.Uint32(buf[*offset:])
	*offset += 4
	return v
}

// readI32 reads a little-endian int32, advancing the offset by 4.
func readI32(buf []byte, offset *int) int32 {
	if *offset+4 > len(buf) {
		*offset += 4
		return 0
	}
	v := int32(binary.LittleEndian.Uint32(buf[*offset:]))
	*offset += 4
	return v
}

// readMask reads a 4-byte little-endian component mask, zero-extending from
// whatever bytes remain. The offset is moved to the end of the input when
// truncated so the entity loop terminates.
func readMask(data []byte, offset *int) uint32 {
	avail := len(data) - *offset
	if avail >= 4 {
		mask := binary.LittleEndian.Uint32(data[*offset:])
		*offset += 4
		return mask
	}
	padded := make([]byte, 4)
	copy(padded, data[*offset:])
	*offset = len(data)
	return binary.LittleEndian.Uint32(padded)
}

// readBytes returns dataLen bytes starting at *offset, zero-padding when the
// input is truncated. The offset advances by dataLen (or to EOF).
func readBytes(data []byte, offset *int, dataLen int) []byte {
	remaining := len(data) - *offset
	if remaining >= dataLen {
		out := data[*offset : *offset+dataLen]
		*offset += dataLen
		return out
	}
	padded := make([]byte, dataLen)
	copy(padded, data[*offset:])
	*offset = len(data)
	return padded
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampU8(v, hi uint8) uint8 {
	if v > hi {
		return hi
	}
	return v
}

// seedFromBytes derives a deterministic seed from the full input so two drivers
// built from the same bytes share an RNG stream. It is a pure hash, never used
// in the simulation arithmetic path.
func seedFromBytes(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

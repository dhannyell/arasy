package scene

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"log/slog"
	"math"
	"slices"
)

// Checksum returns a deterministic hash of all entity positions and velocities.
// Two worlds with identical simulation state always produce the same value.
// Log it at Debug level across runs to verify the simulation stays in sync.
func (w *World) Checksum() uint64 {
	ids := make([]EntityID, 0, len(w.transforms))
	for id := range w.transforms {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	h := fnv.New64a()
	var buf [8]byte

	for _, id := range ids {
		t := w.transforms[id]
		hashF64(h, buf[:], t.Position.X)
		hashF64(h, buf[:], t.Position.Y)

		if v, ok := w.velocities[id]; ok {
			hashF64(h, buf[:], v.Linear.X)
			hashF64(h, buf[:], v.Linear.Y)
		}
	}

	return h.Sum64()
}

func hashF64(h hash.Hash64, buf []byte, v float64) {
	binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
	_, err := h.Write(buf)
	if err != nil {
		slog.Error("Failed to Write hash", err)
		return
	} //nolint:errcheck
}

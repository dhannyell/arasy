package scene

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"log/slog"
	"math"
)

const (
	tagMeta byte = iota
	tagTransform
	tagTransformTween
	tagCircleSprite
	tagCircleSpriteTween
	tagRectSprite
	tagVelocity
	tagAcceleration
	tagDamping
	tagBody
	tagCircleCollider
	tagRectCollider
	tagImageSprite
	tagHeight
	tagSpin
	tagBallistics
	tagLocomotion
	tagMoveIntent
	tagHopTween
)

// ChecksumComponentStores counts the component stores covered by Checksum.
// The structural guard test compares it against reflection over World.
const ChecksumComponentStores = 18

// Checksum returns a deterministic hash of every component store, simulation
// and presentation alike. Two worlds with identical state always produce the
// same value; rewind parity and the structural guard rely on it being
// bit-complete. Log it at Debug level across runs to verify sync.
func (w *World) Checksum() uint64 {
	return w.checksum(true, true)
}

// SimChecksum hashes only simulation state: the stores World.Step reads or
// writes (transforms, tweens, physics, heights, steering) plus entity and
// tick meta. Replay anchors use it, so editing presentation state -- sprite
// colors, geometry -- never invalidates a recording. Tween stores are
// simulation state even when they animate a sprite: the tween's own fields
// carry the deterministic clock, the sprite it writes is derived.
//
// The guarantee assumes simulation code never reads presentation state.
// Game logic that branches on a sprite field diverges silently under
// replay; that discipline belongs to the game, not to this hash.
func (w *World) SimChecksum() uint64 {
	return w.checksum(true, false)
}

// ContentChecksum hashes only presentation state: the sprite stores. It is a
// fingerprint of authored visuals, recorded once per replay so a divergence
// loads with a warning ("the content changed since recording") instead of a
// false anchor failure.
//
// It covers components, not assets: an ImageSprite hashes its Texture
// handle, never the pixels behind it. Swapping a spritesheet in the same
// texture slot changes neither hash.
func (w *World) ContentChecksum() uint64 {
	return w.checksum(false, true)
}

// checksum walks componentDefs in table order with a class guard per store.
// With both classes on, the byte stream is the historical Checksum stream,
// which goldens pin. An entry may change class, never position: grouping by
// class would reorder the stream.
func (w *World) checksum(sim, content bool) uint64 {
	h := fnv.New64a()
	var buf [8]byte

	if sim {
		_, err := h.Write([]byte{tagMeta})
		if err != nil {
			slog.Error("Failed to generate checksum", "err", err.Error())
			return 0
		}
		hashU64(h, buf[:], uint64(w.nextID))
		hashU64(h, buf[:], w.stepTick)
	}
	for i := range componentDefs {
		d := &componentDefs[i]
		if d.hash == nil || (d.sim && !sim) || (d.content && !content) {
			continue
		}
		if d.skipEmpty && d.length(w) == 0 {
			continue
		}
		d.hash(w, h, buf[:])
	}

	return h.Sum64()
}

func hashCircleSprite(h hash.Hash64, buf []byte, s CircleSprite) {
	hashU64(h, buf, uint64(int64(s.Layer)))
	hashF64(h, buf, s.Color.R)
	hashF64(h, buf, s.Color.G)
	hashF64(h, buf, s.Color.B)
	hashF64(h, buf, s.Color.A)
	hashF64(h, buf, s.Radius)
}

func hashComponent[T any](h hash.Hash64, buf []byte, tag byte,
	s *store[T], write func(T)) {
	_, err := h.Write([]byte{tag})
	if err != nil {
		slog.Error("Failed hash component", "err", err.Error())
		return
	}
	for i, id := range s.ids {
		binary.LittleEndian.PutUint64(buf, uint64(id))
		if _, err := h.Write(buf); err != nil {
			slog.Error("Failed hash component", "err", err.Error())
			return
		}
		write(s.dense[i])
	}
}

func hashF64(h hash.Hash64, buf []byte, v float64) {
	binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
	_, err := h.Write(buf)
	if err != nil {
		slog.Error("failed to write hash", "err", err)
		return
	}
}

func hashU64(h hash.Hash64, buf []byte, v uint64) {
	binary.LittleEndian.PutUint64(buf, v)
	_, err := h.Write(buf)
	if err != nil {
		slog.Error("failed to write hash", "err", err)
		return
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}

	return 0.0
}

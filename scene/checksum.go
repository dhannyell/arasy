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

// checksum walks the component stores in the historical order with a class
// guard per store. With both classes on, the byte stream is the historical
// Checksum stream, which goldens pin. A store may change class, never
// position: grouping by class would reorder the stream.
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

		hashComponent(h, buf[:], tagTransform, &w.transforms, func(t Transform) {
			hashF64(h, buf[:], t.Position.X)
			hashF64(h, buf[:], t.Position.Y)
			hashF64(h, buf[:], t.Rotation)
			hashF64(h, buf[:], t.Scale.X)
			hashF64(h, buf[:], t.Scale.Y)
		})
		hashComponent(h, buf[:], tagTransformTween, &w.transformTweens, func(tw TransformTween) {
			hashF64(h, buf[:], tw.From.Rotation)
			hashF64(h, buf[:], tw.From.Scale.X)
			hashF64(h, buf[:], tw.From.Scale.Y)
			hashF64(h, buf[:], tw.From.Position.X)
			hashF64(h, buf[:], tw.From.Position.Y)
			hashF64(h, buf[:], tw.To.Rotation)
			hashF64(h, buf[:], tw.To.Scale.X)
			hashF64(h, buf[:], tw.To.Scale.Y)
			hashF64(h, buf[:], tw.To.Position.X)
			hashF64(h, buf[:], tw.To.Position.Y)
			hashF64(h, buf[:], tw.Duration)
			hashF64(h, buf[:], tw.Elapsed)
			hashU64(h, buf[:], uint64(tw.Ease))
			hashU64(h, buf[:], uint64(tw.Loop))
			hashF64(h, buf[:], boolToFloat(tw.Done))
			hashF64(h, buf[:], boolToFloat(tw.RemoveOnComplete))
		})
	}
	if content {
		hashComponent(h, buf[:], tagCircleSprite, &w.circleSprites, func(s CircleSprite) {
			hashU64(h, buf[:], uint64(int64(s.Layer)))
			hashF64(h, buf[:], s.Color.R)
			hashF64(h, buf[:], s.Color.G)
			hashF64(h, buf[:], s.Color.B)
			hashF64(h, buf[:], s.Color.A)
			hashF64(h, buf[:], s.Radius)
		})
	}
	if sim {
		hashComponent(h, buf[:], tagCircleSpriteTween, &w.circleTweens, func(tw CircleSpriteTween) {
			hashCircleSprite(h, buf[:], tw.From)
			hashCircleSprite(h, buf[:], tw.To)
			hashF64(h, buf[:], tw.Duration)
			hashF64(h, buf[:], tw.Elapsed)
			hashU64(h, buf[:], uint64(tw.Ease))
			hashU64(h, buf[:], uint64(tw.Loop))
			hashF64(h, buf[:], boolToFloat(tw.Done))
			hashF64(h, buf[:], boolToFloat(tw.RemoveOnComplete))
		})
	}
	if content {
		hashComponent(h, buf[:], tagRectSprite, &w.rectSprites, func(s RectSprite) {
			hashU64(h, buf[:], uint64(int64(s.Layer)))
			hashF64(h, buf[:], s.Color.R)
			hashF64(h, buf[:], s.Color.G)
			hashF64(h, buf[:], s.Color.B)
			hashF64(h, buf[:], s.Color.A)
			hashF64(h, buf[:], s.Rect.Min.X)
			hashF64(h, buf[:], s.Rect.Min.Y)
			hashF64(h, buf[:], s.Rect.Max.X)
			hashF64(h, buf[:], s.Rect.Max.Y)
			hashF64(h, buf[:], s.StrokeWidth)
			hashF64(h, buf[:], s.Stroke.R)
			hashF64(h, buf[:], s.Stroke.G)
			hashF64(h, buf[:], s.Stroke.B)
			hashF64(h, buf[:], s.Stroke.A)
		})
	}
	if sim {
		hashComponent(h, buf[:], tagVelocity, &w.velocities, func(v Velocity) {
			hashF64(h, buf[:], v.Linear.X)
			hashF64(h, buf[:], v.Linear.Y)
		})
		hashComponent(h, buf[:], tagAcceleration, &w.accelerations, func(a Acceleration) {
			hashF64(h, buf[:], a.Linear.X)
			hashF64(h, buf[:], a.Linear.Y)
		})
		hashComponent(h, buf[:], tagDamping, &w.dampings, func(d Damping) {
			hashF64(h, buf[:], d.Linear)
		})
		hashComponent(h, buf[:], tagBody, &w.bodies, func(b Body) {
			hashF64(h, buf[:], b.InvMass)
			hashF64(h, buf[:], b.Restitution)
		})
		hashComponent(h, buf[:], tagCircleCollider, &w.circleColliders, func(c CircleCollider) {
			hashF64(h, buf[:], c.Radius)
		})
		hashComponent(h, buf[:], tagRectCollider, &w.rectColliders, func(c RectCollider) {
			hashF64(h, buf[:], c.HalfExtends.X)
			hashF64(h, buf[:], c.HalfExtends.Y)
		})
		hashComponent(h, buf[:], tagHeight, &w.heights, func(h2 Height) {
			hashF64(h, buf[:], h2.Z)
			hashF64(h, buf[:], h2.VZ)
		})
		hashComponent(h, buf[:], tagSpin, &w.spins, func(s Spin) {
			hashF64(h, buf[:], s.Omega)
		})
		hashComponent(h, buf[:], tagBallistics, &w.ballistics, func(b Ballistics) {
			hashF64(h, buf[:], b.Gravity)
			hashF64(h, buf[:], b.DragK)
			hashF64(h, buf[:], b.MagnusK)
			hashF64(h, buf[:], b.BounceDamping)
			hashF64(h, buf[:], b.BounceFriction)
			hashF64(h, buf[:], b.SpinDecay)
			hashF64(h, buf[:], b.RollFriction)
			hashF64(h, buf[:], b.RestVZ)
			hashU64(h, buf[:], uint64(int64(b.Substeps)))
		})
		hashComponent(h, buf[:], tagLocomotion, &w.locomotions, func(l Locomotion) {
			hashF64(h, buf[:], l.Accel)
			hashF64(h, buf[:], l.Decel)
			hashF64(h, buf[:], l.MaxSpeed)
			hashF64(h, buf[:], l.Agility)
		})
		hashComponent(h, buf[:], tagMoveIntent, &w.moveIntents, func(m MoveIntent) {
			hashF64(h, buf[:], m.Target.X)
			hashF64(h, buf[:], m.Target.Y)
			hashF64(h, buf[:], m.DesiredSpeed)
		})
		hashComponent(h, buf[:], tagHopTween, &w.hopTweens, func(tw HopTween) {
			hashF64(h, buf[:], tw.From.X)
			hashF64(h, buf[:], tw.From.Y)
			hashF64(h, buf[:], tw.To.X)
			hashF64(h, buf[:], tw.To.Y)
			hashF64(h, buf[:], tw.Apex)
			hashF64(h, buf[:], tw.Duration)
			hashF64(h, buf[:], tw.Elapsed)
		})
	}
	if content && w.imageSprites.len() > 0 {
		hashComponent(h, buf[:], tagImageSprite, &w.imageSprites, func(s ImageSprite) {
			hashU64(h, buf[:], uint64(int64(s.Layer)))
			hashU64(h, buf[:], uint64(s.Texture))
			hashF64(h, buf[:], s.Src.Min.X)
			hashF64(h, buf[:], s.Src.Min.Y)
			hashF64(h, buf[:], s.Src.Max.X)
			hashF64(h, buf[:], s.Src.Max.Y)
			hashF64(h, buf[:], s.Size.X)
			hashF64(h, buf[:], s.Size.Y)
			hashF64(h, buf[:], s.Tint.R)
			hashF64(h, buf[:], s.Tint.G)
			hashF64(h, buf[:], s.Tint.B)
			hashF64(h, buf[:], s.Tint.A)
			hashF64(h, buf[:], s.ShadowFootprint.X)
			hashF64(h, buf[:], s.ShadowFootprint.Y)
		})
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

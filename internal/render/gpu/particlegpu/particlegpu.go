// Package particlegpu owns the backend-independent particle GPU layout.
package particlegpu

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

const (
	// HotStride is the byte stride of the frequently updated particle data.
	HotStride = 24
	// ColdStride is the byte stride of the stable particle data.
	ColdStride = 32
	// UniformSize is the byte size of the emitter uniform block.
	UniformSize = 240
	// Workgroup is the compute workgroup size used by the particle pass.
	Workgroup = 64
	// DrawArgsSize is the byte size of the indirect draw arguments.
	DrawArgsSize = 16
	// CountersAliveOffset is the byte offset of the live-particle counter.
	CountersAliveOffset = 4
	// DrawInstanceOffset is the byte offset of the instance count in draw args.
	DrawInstanceOffset = 4
	// TTL is the number of frames kept by the particle timing ring.
	TTL = 300
)

// EmitterKey identifies one emitter in a particle effect.
type EmitterKey struct {
	Effect  render.ParticleEffectID
	Emitter render.EmitterID
}

type uniformWriter struct {
	dst    []byte
	offset int
}

func (w *uniformWriter) f32(v float64) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], math.Float32bits(float32(v)))
	w.offset += 4
}
func (w *uniformWriter) u32(v uint32) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], v)
	w.offset += 4
}
func (w *uniformWriter) vec2(v math2d.Vec2) { w.f32(v.X); w.f32(v.Y) }
func (w *uniformWriter) vec4(v [4]float64) {
	for _, c := range v {
		w.f32(c)
	}
}

// PackUniforms writes the emitter data in the layout expected by WGSL.
func PackUniforms(dst []byte, d particle.EffectDecl, cmd render.DrawParticles, frame particle.EmissionFrame, emitCount int, seed uint64, capacity int, camera render.Camera, width, height int) {
	if len(dst) < UniformSize {
		panic("particle uniform buffer is too small")
	}
	worldToClip, offset, screenScale, screenToClipScale := tess.WorldToClip(camera, width, height)
	wobbleAxis := math2d.Vec2{}
	if d.WobbleAmp != 0 {
		angle := d.Direction + cmd.Dir
		wobbleAxis = math2d.Vec2{X: -math.Sin(angle), Y: math.Cos(angle)}
	}
	w := uniformWriter{dst: dst}
	w.vec4(d.ColorStart)
	w.vec4(d.ColorEnd)
	w.vec2(worldToClip[0])
	w.vec2(worldToClip[1])
	w.vec2(offset)
	w.vec2(screenScale)
	w.vec2(screenToClipScale)
	w.vec2(cmd.Pos)
	w.vec2(frame.PrevPos)
	w.vec2(d.Gravity)
	w.vec2(d.EmitExtent)
	w.vec2(EmitScale(cmd.EmitScale))
	w.vec2(wobbleAxis)
	w.f32(cmd.Dt)
	w.f32(d.Drag)
	w.u32(uint32(emitCount))
	w.u32(frame.Base)
	w.u32(uint32(seed))
	w.u32(uint32(seed >> 32))
	w.f32(d.Direction + cmd.Dir)
	w.f32(d.Spread)
	w.f32(d.Speed.Min)
	w.f32(d.Speed.Max)
	w.f32(d.Lifetime.Min)
	w.f32(d.Lifetime.Max)
	w.f32(d.SizeStart.Min)
	w.f32(d.SizeStart.Max)
	w.f32(d.SizeEnd.Min)
	w.f32(d.SizeEnd.Max)
	w.f32(d.Spin)
	w.f32(d.Rotation.Min)
	w.f32(d.Rotation.Max)
	w.f32(d.WobbleAmp)
	w.f32(d.WobbleFreq)
	w.f32(d.FadeIn)
	w.f32(d.FlipXFreq)
	w.u32(uint32(d.Shape))
	w.u32(uint32(d.VelMode))
	w.f32(d.EmitRadius)
	w.u32(uint32(capacity))
	// WebGPU rounds this uniform struct up to a 16-byte boundary. Keep the
	// trailing padding deterministic when the scratch buffer is reused.
	for w.offset < UniformSize {
		w.dst[w.offset] = 0
		w.offset++
	}
}

// EmitScale replaces a zero component with the default scale of one.
func EmitScale(scale math2d.Vec2) math2d.Vec2 {
	if scale.X == 0 {
		scale.X = 1
	}
	if scale.Y == 0 {
		scale.Y = 1
	}
	return scale
}

// ScissorRect converts a world clip to a pixel scissor rectangle.
func ScissorRect(clip math2d.Rect, camera render.Camera, scale float64, width, height int) (x, y, w, h int, ok bool) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, false
	}
	if clip.Width() <= 0 || clip.Height() <= 0 {
		return 0, 0, width, height, true
	}
	minPoint, maxPoint := camera.WorldToScreen(clip.Min), camera.WorldToScreen(clip.Max)
	x0 := max(0, min(width, int(math.Floor(math.Min(minPoint.X, maxPoint.X)*scale))))
	y0 := max(0, min(height, int(math.Floor(math.Min(minPoint.Y, maxPoint.Y)*scale))))
	x1 := max(0, min(width, int(math.Ceil(math.Max(minPoint.X, maxPoint.X)*scale))))
	y1 := max(0, min(height, int(math.Ceil(math.Max(minPoint.Y, maxPoint.Y)*scale))))
	if x1 <= x0 || y1 <= y0 {
		return 0, 0, width, height, false
	}
	return x0, y0, x1 - x0, y1 - y0, true
}

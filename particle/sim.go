package particle

import (
	"math"
	"math/bits"

	"github.com/dhannyell/frameforge/math2d"
)

// Emitter is the CPU-side state of one live effect instance. Software
// backends own a map[<EmitterKey>]*Emitter; the GPU backend mirrors this
// semantics in compute shaders.
type Emitter struct {
	Decl      EffectDecl
	Seed      uint64
	pool      []Particle // len == capacity after Init
	free      []int32    // dead-slot stack
	aliveBits []uint64   // one bit per live pool slot
	emission  EmissionState
}

type Particle struct {
	Pos, Vel           math2d.Vec2
	Age, Life          float64
	SizeStart, SizeEnd float64
	Spin               float64

	Rotation    float64
	WobblePhase float64
	WobbleAxis  math2d.Vec2

	Hash [3]uint32 // spawn hash, reused for color/size jitter
}

type LiveParticle struct {
	Pos      math2d.Vec2
	Rotation float64
	Size     float64
	Color    [4]float64
	Progress float64 // Age/Life in [0,1); drives size/color interpolation.
}

// NewEmitter builds the CPU-side state for one effect instance. The pool,
// free list, and active-slot bitset are allocated once here; Step and Alive
// never allocate.
func NewEmitter(decl EffectDecl, seed uint64) *Emitter {
	c := min(max(decl.Capacity, 1), MaxCapacity)
	decl.Capacity = c
	e := &Emitter{
		Decl:      decl,
		Seed:      seed,
		pool:      make([]Particle, c),
		free:      make([]int32, c),
		aliveBits: make([]uint64, (c+63)/64),
	}
	for i := range e.free {
		e.free[i] = int32(c - 1 - i)
	}
	return e
}

// Step advances the emitter by dt.
// Emission runs before integration so newborns age dt on their birth frame, matching the GPU
// pass order (emit before simulate). maxAlive caps the pool actually used
// this frame (budget); emission past the cap is dropped, not deferred.
// dt <= 0 neither emits nor integrates (pause/scrub).
func (e *Emitter) Step(pos math2d.Vec2, dirOffset, intensity float64, burst uint32, dt float64, maxAlive int) {
	frame := e.emission.Advance(pos, e.Decl.EmitRate, intensity, e.Decl.BurstSize, burst, dt)
	if dt <= 0 {
		return
	}

	maxAlive = min(max(maxAlive, 0), len(e.pool))
	alive := len(e.pool) - len(e.free)
	frame.Count = min(frame.Count, max(0, maxAlive-alive))

	for n := 0; n < frame.Count; n++ {
		t := float64(n+1) / float64(frame.Count)
		origin := frame.PrevPos.Lerp(pos, t)
		e.spawn(origin, dirOffset, frame.Base+uint32(n))
	}
	if frame.Count > 0 {
		e.emission.Emitted += uint32(frame.Count)
	}

	gx, gy := e.Decl.Gravity.X*dt, e.Decl.Gravity.Y*dt
	drag := math.Pow(e.Decl.Drag, dt)

	for wordIndex, active := range e.aliveBits {
		for active != 0 {
			bitIndex := bits.TrailingZeros64(active)
			slot := wordIndex*64 + bitIndex
			p := &e.pool[slot]

			p.Age += dt
			if p.Age >= p.Life {
				e.markDead(slot)
				e.free = append(e.free, int32(slot))
				active &= active - 1
				continue
			}
			p.Vel.X = (p.Vel.X + gx) * drag
			p.Vel.Y = (p.Vel.Y + gy) * drag
			p.Pos.X += p.Vel.X * dt
			p.Pos.Y += p.Vel.Y * dt
			active &= active - 1
		}
	}
}

// AliveCount returns the number of currently live particles in O(1).
// Backends can use it to size transient presentation buffers to occupancy
// instead of the emitter's full capacity.
func (e *Emitter) AliveCount() int {
	return len(e.pool) - len(e.free)
}

// Alive appends the live particles to dst and returns it, in stable slot order
func (e *Emitter) Alive(dst []LiveParticle) []LiveParticle {
	for wordIndex, active := range e.aliveBits {
		for active != 0 {
			bitIndex := bits.TrailingZeros64(active)
			slot := wordIndex*64 + bitIndex
			p := &e.pool[slot]

			progress := p.Age / p.Life
			size := math2d.Lerp(p.SizeStart, p.SizeEnd, progress)

			rotation := p.Rotation + p.Spin*p.Age

			color := [4]float64{
				math2d.Lerp(e.Decl.ColorStart[0], e.Decl.ColorEnd[0], progress),
				math2d.Lerp(e.Decl.ColorStart[1], e.Decl.ColorEnd[1], progress),
				math2d.Lerp(e.Decl.ColorStart[2], e.Decl.ColorEnd[2], progress),
				math2d.Lerp(e.Decl.ColorStart[3], e.Decl.ColorEnd[3], progress),
			}

			if e.Decl.FadeIn > 0 {
				fade := min(1.0, progress/e.Decl.FadeIn)
				color[3] *= fade
			}

			pos := p.Pos
			if e.Decl.WobbleAmp != 0 && e.Decl.WobbleFreq != 0 {
				phase := 2*math.Pi*e.Decl.WobbleFreq*p.Age + p.WobblePhase
				offset := e.Decl.WobbleAmp * math.Sin(phase)
				pos = pos.Add(p.WobbleAxis.Mul(offset))
			}

			live := LiveParticle{
				Pos:      pos,
				Rotation: rotation,
				Color:    color,
				Size:     size,
				Progress: math2d.Clamp(progress, 0, 1),
			}

			dst = append(dst, live)
			active &= active - 1
		}
	}

	return dst
}

// Spawn initializes one particle in a free slot. Every spawn attribute is
// a pure function of (emitted, Seed) — never of the slot — so the GPU emit
// pass spawns identical particles for the same counter. The lane layout
// below must mirror the particle WGSL shader exactly.
func (e *Emitter) spawn(origin math2d.Vec2, dirOffset float64, emitted uint32) {
	h1 := pcg3d([3]uint32{emitted, uint32(e.Seed), uint32(e.Seed >> 32)})
	h2 := pcg3d(h1)
	h3 := pcg3d(h2)
	h4 := pcg3d(h3)

	slot := e.free[len(e.free)-1]
	e.free = e.free[:len(e.free)-1]

	d := &e.Decl
	offset := spawnOffset(d, h3)

	e.pool[slot] = Particle{
		Pos:         origin.Add(offset),
		Vel:         initialVelocity(d, offset, dirOffset, h1),
		Life:        math2d.Lerp(d.Lifetime.Min, d.Lifetime.Max, Float01(h1[2])),
		SizeStart:   math2d.Lerp(d.SizeStart.Min, d.SizeStart.Max, Float01(h2[0])),
		SizeEnd:     math2d.Lerp(d.SizeEnd.Min, d.SizeEnd.Max, Float01(h2[1])),
		Spin:        (2*Float01(h2[2]) - 1) * d.Spin,
		Rotation:    math2d.Lerp(d.Rotation.Min, d.Rotation.Max, Float01(h4[0])),
		WobblePhase: 2 * math.Pi * Float01(h4[1]),
		Hash:        h1,
	}

	if d.WobbleAmp != 0 {
		baseAngle := d.Direction + dirOffset
		e.pool[slot].WobbleAxis = math2d.Vec2{
			X: -math.Sin(baseAngle),
			Y: math.Cos(baseAngle),
		}
	}
	e.markAlive(int(slot))
}

func (e *Emitter) markAlive(slot int) {
	e.aliveBits[slot/64] |= uint64(1) << uint(slot%64)
}

func (e *Emitter) markDead(slot int) {
	e.aliveBits[slot/64] &^= uint64(1) << uint(slot%64)
}

func spawnOffset(d *EffectDecl, h3 [3]uint32) math2d.Vec2 {
	switch d.Shape {
	case EmitDisc:
		radius := d.EmitRadius * math.Sqrt(Float01(h3[0]))
		angle := 2 * math.Pi * Float01(h3[1])
		return math2d.Vec2{
			X: math.Cos(angle) * radius,
			Y: math.Sin(angle) * radius,
		}
	case EmitRect:
		return math2d.Vec2{
			X: (2*Float01(h3[0]) - 1) * d.EmitExtent.X,
			Y: (2*Float01(h3[1]) - 1) * d.EmitExtent.Y,
		}
	case EmitRing:
		angle := 2 * math.Pi * Float01(h3[1])
		return math2d.Vec2{
			X: math.Cos(angle) * d.EmitRadius,
			Y: math.Sin(angle) * d.EmitRadius,
		}
	default: // EmitPoint
		return math2d.Vec2{}
	}
}

func initialVelocity(d *EffectDecl, offset math2d.Vec2, dirOffset float64, h1 [3]uint32) math2d.Vec2 {
	speed := math2d.Lerp(d.Speed.Min, d.Speed.Max, Float01(h1[1]))
	baseAngle := d.Direction + dirOffset

	var direction math2d.Vec2
	switch d.VelMode {
	case VelTangential:
		baseDir := math2d.Vec2{
			X: math.Cos(baseAngle),
			Y: math.Sin(baseAngle),
		}
		radial := offset.Normalize()
		if radial.LenSq() == 0 {
			radial = baseDir
		}
		direction = math2d.Vec2{
			X: -radial.Y,
			Y: radial.X,
		}
	case VelRadial:
		baseDir := math2d.Vec2{
			X: math.Cos(baseAngle),
			Y: math.Sin(baseAngle),
		}
		direction = offset.Normalize()
		if direction.LenSq() == 0 {
			direction = baseDir
		}
	default:
		// VelCone and any out-of-range value fall back to the cone.
		angle := baseAngle + (2*Float01(h1[0])-1)*d.Spread
		direction = math2d.Vec2{
			X: math.Cos(angle),
			Y: math.Sin(angle),
		}
	}

	return direction.Mul(speed)
}

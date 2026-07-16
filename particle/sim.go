package particle

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
)

// Emitter is the CPU-side state of one live effect instance. Software
// backends own a map[<EmitterKey>]*Emitter; the GPU backend mirrors this
// semantics in compute shaders.
type Emitter struct {
	Decl      EffectDecl
	Seed      uint64
	pool      []Particle  // len == capacity after Init
	free      []int32     // dead-slot stack
	emitAcc   float64     // fractional emission accumulator
	emitted   uint32      // monotonically increasing spawn counter (hash lane)
	lastBurst uint32      // last seen cumulative burst counter
	lastPos   math2d.Vec2 // emitter position on the previous Step (spawn lerp)
	started   bool
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
}

// NewEmitter builds the CPU-side state for one effect instance. The pool
// and free list are allocated once here; Step and Alive never allocate.
func NewEmitter(decl EffectDecl, seed uint64) *Emitter {
	c := min(max(decl.Capacity, 1), MaxCapacity)
	decl.Capacity = c
	e := &Emitter{
		Decl: decl,
		Seed: seed,
		pool: make([]Particle, c),
		free: make([]int32, c),
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
	if !e.started {
		e.lastBurst = burst
		e.lastPos = pos
		e.started = true
	}

	previousPos := e.lastPos

	if dt <= 0.0 {
		e.lastPos = pos
		return
	}

	// Emission accounting. The GPU backend keeps this exact arithmetic on
	// the Go side to size its emit dispatch.
	e.emitAcc += e.Decl.EmitRate * intensity * dt
	toEmit := int(e.emitAcc)
	e.emitAcc -= float64(toEmit)

	if burst > e.lastBurst {
		toEmit += int(burst-e.lastBurst) * e.Decl.BurstSize
		e.lastBurst = burst
	}

	maxAlive = min(max(maxAlive, 0), len(e.pool))
	alive := len(e.pool) - len(e.free)
	toEmit = min(toEmit, max(0, maxAlive-alive))

	for n := 0; n < toEmit; n++ {
		t := float64(n+1) / float64(toEmit)
		origin := previousPos.Lerp(pos, t)
		e.spawn(origin, dirOffset)
	}

	// Must update even when toEmit == 0
	e.lastPos = pos

	gx, gy := e.Decl.Gravity.X*dt, e.Decl.Gravity.Y*dt
	drag := math.Pow(e.Decl.Drag, dt)

	for i := range e.pool {
		p := &e.pool[i]
		if p.Age >= p.Life {
			continue
		}

		p.Age += dt
		if p.Age >= p.Life {
			e.free = append(e.free, int32(i))
			continue
		}
		p.Vel.X = (p.Vel.X + gx) * drag
		p.Vel.Y = (p.Vel.Y + gy) * drag
		p.Pos.X += p.Vel.X * dt
		p.Pos.Y += p.Vel.Y * dt
	}
}

// Alive appends the live particles to dst and returns it, in stable slot order
func (e *Emitter) Alive(dst []LiveParticle) []LiveParticle {
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age >= p.Life {
			continue
		}

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
		}

		dst = append(dst, live)
	}

	return dst
}

// Spawn initializes one particle in a free slot. Every spawn attribute is
// a pure function of (emitted, Seed) — never of the slot — so the GPU emit
// pass spawns identical particles for the same counter. The lane layout
// below must mirror the particle WGSL shader exactly.
func (e *Emitter) spawn(origin math2d.Vec2, dirOffset float64) {
	h1 := pcg3d([3]uint32{e.emitted, uint32(e.Seed), uint32(e.Seed >> 32)})
	h2 := pcg3d(h1)
	h3 := pcg3d(h2)
	h4 := pcg3d(h3)
	e.emitted++

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

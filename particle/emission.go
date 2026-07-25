package particle

import "github.com/dhannyell/arasy/math2d"

// EmissionState is the per-emitter CPU accounting shared by every backend:
// the fractional emission accumulator, the cumulative burst tracker and the
// previous emitter position for spawn-origin interpolation. Software
// backends drive it inside Emitter.Step; the GPU backend runs it standalone
// to size its emit dispatch and fill the frame uniforms, so both spawn the
// same particle sequence for the same command stream.
type EmissionState struct {
	Acc       float64
	Emitted   uint32 // monotonic spawn counter; the hash lane base
	LastBurst uint32
	LastPos   math2d.Vec2
	started   bool
}

// EmissionFrame is one Advance result: how many particles to spawn, the
// spawn-counter base for the first of them, and the origin-lerp start.
type EmissionFrame struct {
	Count   int
	Base    uint32
	PrevPos math2d.Vec2
}

// Advance accounts one frame. dt <= 0 emits nothing but still tracks the
// emitter position (pause/scrub). The first call seeds LastBurst/LastPos
// without firing a retroactive burst. Advance does not consume Emitted:
// the caller advances it by the number of particles actually spawned.
// A CPU pool may clamp below Count; a GPU emit pass consumes the full range
// even when a full pool drops some spawns, keeping later hash lanes stable.
func (s *EmissionState) Advance(
	pos math2d.Vec2,
	rate,
	intensity float64,
	burstSize int,
	burst uint32,
	dt float64,
) EmissionFrame {
	if !s.started {
		s.LastBurst = burst
		s.LastPos = pos
		s.started = true
	}
	frame := EmissionFrame{Base: s.Emitted, PrevPos: s.LastPos}
	if dt <= 0 {
		s.LastPos = pos
		return frame
	}

	s.Acc += rate * intensity * dt
	frame.Count = int(s.Acc)
	s.Acc -= float64(frame.Count)

	if burst > s.LastBurst {
		frame.Count += int(burst-s.LastBurst) * burstSize
		s.LastBurst = burst
	}
	s.LastPos = pos
	return frame
}

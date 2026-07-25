package particle

import "github.com/dhannyell/arasy/math2d"

// InstanceKey identifies one live effect instance. It mirrors the render
// command's (effect, emitter) pair without importing the render package.
type InstanceKey struct {
	Effect  uint32
	Emitter uint32
}

// EmitterTTL is how many frames an instance survives without being drawn
// before the set releases its state. All CPU backends share this value.
const EmitterTTL = 300

// DiagnosticEffectMissing and DiagnosticBudgetReduced are the shared
// diagnostic codes hosts use with internal/diagnostic when reporting
// particle conditions. Defined here so backends stop duplicating strings.
const (
	DiagnosticEffectMissing = "particle.effect_missing"
	DiagnosticBudgetReduced = "particle.budget_reduced"
)

// Input carries the per-command drive scalars (a render.DrawParticles
// without the render types).
type Input struct {
	Pos       math2d.Vec2
	Dir       float64
	Intensity float64
	Burst     uint32
	Dt        float64
	Seed      uint64
}

// Result is what a backend draws. Live aliases an internal scratch buffer:
// it is valid only until the next Draw or BeginFrame call.
type Result struct {
	Live          []LiveParticle
	BudgetReduced bool // this instance got less than decl.Capacity this frame
	Skipped       bool // conflicting seed within the frame; draw nothing
}

type setEntry struct {
	sim      *Emitter
	lastSeen uint64
}

// EmitterSet owns the CPU-side lifecycle of every live effect instance:
// keyed state, seed resets, first-command-wins frame dedupe, TTL eviction,
// scratch reuse and the optional global budget. Backends draw Result.Live
// and keep no particle state of their own.
type EmitterSet struct {
	budget                *Budget
	emitters              map[InstanceKey]*setEntry
	scratch               []LiveParticle
	frame, frameSeen      uint64
	wantPrev, wantCurrent int
}

// NewEmitterSet creates a set. A nil budget means no global cap: every
// instance may reach its declared capacity (the raster/offline policy —
// deterministic output; wall-clock AIMD belongs to interactive hosts).
func NewEmitterSet(budget *Budget) *EmitterSet {
	return &EmitterSet{budget: budget, emitters: make(map[InstanceKey]*setEntry)}
}

// BeginFrame advances the frame counter and evicts instances not drawn for
// EmitterTTL frames. Hosts call it once per rendered frame, before issuing
// that frame's Draw calls.
//
// Eviction is checked against the frame counter's value from before this
// call's own increment, i.e. the previous frame's number. That frame's own
// Draw calls already ran (they happen after its BeginFrame), so their
// lastSeen updates are visible here; this reproduces evicting at the tail
// of a frame's processing (same-frame redraws rescue an instance) without
// needing a second host-side call after the frame's commands.
func (s *EmitterSet) BeginFrame() {
	for k, e := range s.emitters {
		if s.frame-e.lastSeen >= EmitterTTL {
			delete(s.emitters, k)
		}
	}
	s.frame++
}

// Draw advances (or re-reads, for same-frame duplicates) the instance for
// key and returns the live particles to render.
func (s *EmitterSet) Draw(key InstanceKey, decl EffectDecl, in Input) Result {
	entry := s.emitters[key]

	if entry != nil && entry.lastSeen == s.frame {
		// First command for this key already owns simulation this frame.
		// A matching later command may draw the same particles again in
		// painter order; a conflicting seed cannot replace it.
		if entry.sim.Seed != in.Seed {
			return Result{Skipped: true}
		}
	} else {
		if entry == nil || entry.sim.Seed != in.Seed {
			entry = &setEntry{sim: NewEmitter(decl, in.Seed)}
			s.emitters[key] = entry
		}
		entry.lastSeen = s.frame

		if s.frameSeen != s.frame {
			s.frameSeen = s.frame
			s.wantPrev = s.wantCurrent
			s.wantCurrent = 0
			if s.budget != nil {
				s.budget.SetCeiling(max(s.wantPrev, s.budget.Floor()))
				s.budget.Observe(in.Dt)
			}
		}
		s.wantCurrent += decl.Capacity

		maxAlive := decl.Capacity
		if s.budget != nil {
			maxAlive = s.budget.Share(decl.Capacity, max(s.wantPrev, decl.Capacity))
		}

		entry.sim.Step(in.Pos, in.Dir, in.Intensity, in.Burst, in.Dt, maxAlive)

		return s.collect(entry, maxAlive < decl.Capacity)
	}

	return s.collect(entry, false)
}

func (s *EmitterSet) collect(entry *setEntry, budgetReduced bool) Result {
	n := entry.sim.AliveCount()
	if cap(s.scratch) < n {
		s.scratch = make([]LiveParticle, 0, n)
	}
	s.scratch = entry.sim.Alive(s.scratch[:0])
	return Result{Live: s.scratch, BudgetReduced: budgetReduced}
}

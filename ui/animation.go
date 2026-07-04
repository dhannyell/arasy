package ui

import (
	"math"

	"github.com/dhannyell/frameforge/ease"
)

const animGCMaxAge = 120

// springMaxDt clamps the Spring dt to prevent explosion after frame hitches.
const springMaxDt = 1.0 / 30

// AnimStore retains per-id animation state across render frames.
// It is owned by the runtime driver and lives outside rewind snapshots on
// purpose, so simulation rewind does not restore cosmetic animation state.
type AnimStore struct {
	entries    map[string]animEntry
	frameStamp uint64
}

type animEntry struct {
	value     float64 // Approach/Spring: current position
	velocity  float64 // Spring: current velocity
	startTime float64 // Tween/Clock/SequenceKeyed: render-time at trigger
	key       uint64  // SequenceKeyed: last seen trigger key
	lastFrame uint64
}

// SpringConfig controls the spring simulation parameters.
type SpringConfig struct {
	Stiffness float64 // spring constant (force per unit displacement)
	Damping   float64 // damping coefficient (force per unit velocity)
}

// DefaultSpringConfig returns a critically-damped spring (no overshoot).
func DefaultSpringConfig() SpringConfig {
	const stiffness = 200
	return SpringConfig{Stiffness: stiffness, Damping: 2 * math.Sqrt(stiffness)}
}

// AdvanceFrame increments the frame counter and sweeps entries not accessed in
// the last animGCMaxAge frames. The runtime driver calls it exactly once at the
// start of each render frame, before any Approach/Spring/Tween calls.
func (s *AnimStore) AdvanceFrame() {
	s.frameStamp++
	if s.entries == nil || s.frameStamp <= animGCMaxAge {
		return
	}
	cutoff := s.frameStamp - animGCMaxAge
	for id, e := range s.entries {
		if e.lastFrame < cutoff {
			delete(s.entries, id)
		}
	}
}

func (s *AnimStore) get(id string) (animEntry, bool) {
	if s.entries == nil {
		return animEntry{}, false
	}
	e, ok := s.entries[id]
	return e, ok
}

func (s *AnimStore) set(id string, e animEntry) {
	if s.entries == nil {
		s.entries = make(map[string]animEntry)
	}
	e.lastFrame = s.frameStamp
	s.entries[id] = e
}

// Approach exponentially decays the stored value toward target at the given
// speed (1/s). Frame-rate independent: same visual result at any FPS.
// speed<=0 or dt==0 (e.g. Frame(0)/Commands()) produces an instant jump.
func (u *UI) Approach(id string, target, speed float64) float64 {
	if u.store == nil {
		return target
	}
	e, _ := u.store.get(id)
	if speed <= 0 || u.dt <= 0 {
		e.value = target
		u.store.set(id, e)
		return target
	}
	e.value = target + (e.value-target)*math.Exp(-speed*u.dt)
	u.store.set(id, e)
	return e.value
}

// Spring simulates a damped spring toward target. The dt is clamped at
// springMaxDt to prevent numerical explosion after a hitch.
// Uses semi-implicit Euler: velocity is updated before position.
func (u *UI) Spring(id string, target float64, cfg SpringConfig) float64 {
	if u.store == nil {
		return target
	}
	if cfg.Stiffness == 0 && cfg.Damping == 0 {
		cfg = DefaultSpringConfig()
	}
	e, _ := u.store.get(id)
	dt := u.dt
	if dt > springMaxDt {
		dt = springMaxDt
	}
	acc := cfg.Stiffness*(target-e.value) - cfg.Damping*e.velocity
	e.velocity += acc * dt
	e.value += e.velocity * dt
	u.store.set(id, e)
	return e.value
}

// Tween returns the ease-shaped progress from 0 to 1 over dur seconds,
// saturating at 1 after the duration elapses. Starts on first call.
func (u *UI) Tween(id string, dur float64, k ease.Kind) float64 {
	if u.store == nil {
		return 1
	}
	e, ok := u.store.get(id)
	if !ok {
		e.startTime = u.time
	}
	u.store.set(id, e)
	t := ease.Progress(u.time-e.startTime, dur)
	return ease.Apply(k, t)
}

// Clock returns the elapsed seconds since the entry's first appearance.
func (u *UI) Clock(id string) float64 {
	if u.store == nil {
		return 0
	}
	e, ok := u.store.get(id)
	if !ok {
		e.startTime = u.time
	}
	u.store.set(id, e)
	return u.time - e.startTime
}

// Reset restarts the Tween/Clock for id and zeroes Approach/Spring state.
func (u *UI) Reset(id string) {
	if u.store == nil {
		return
	}
	u.store.set(id, animEntry{startTime: u.time})
}

// Sequence is an animation clock returned by SequenceKeyed.
// A zero Sequence (key=0, never triggered) always reports Done.
type Sequence struct {
	elapsed float64
	active  bool
}

// Done reports whether the sequence has reached or passed dur seconds.
// A never-triggered sequence (key=0) always returns true so callers can
// skip rendering with a simple early-out guard.
func (s Sequence) Done(dur float64) bool {
	return !s.active || s.elapsed >= dur
}

// Track returns a Track for the window [start, end] seconds in the sequence
// with easing k applied. Values outside the window are clamped at 0 or 1.
func (s Sequence) Track(start, end float64, k ease.Kind) Track {
	if !s.active {
		return Track{}
	}
	dur := end - start
	if dur <= 0 {
		return Track{value: 1, isDone: true}
	}
	t := ease.Progress(s.elapsed-start, dur)
	return Track{value: ease.Apply(k, t), isDone: s.elapsed >= end}
}

// Track is a single eased window within a Sequence.
type Track struct {
	value  float64
	isDone bool
}

// Value returns the eased progress t∈[0,1] for this track's window.
func (tr Track) Value() float64 { return tr.value }

// Lerp interpolates linearly from a to b using Value().
func (tr Track) Lerp(a, b float64) float64 { return a + (b-a)*tr.value }

// Done reports whether the sequence clock has passed this track's end time.
func (tr Track) Done() bool { return tr.isDone }

// SequenceKeyed returns a Sequence that (re)starts whenever key changes.
// key=0 means "never triggered": Done() always returns true, Track returns
// zero values. On simulation rewind, if key reverts the sequence re-fires
// when the event recurs — intentional for cosmetic animations.
func (u *UI) SequenceKeyed(id string, key uint64) Sequence {
	if u.store == nil {
		return Sequence{}
	}
	e, _ := u.store.get(id)
	if key == 0 {
		u.store.set(id, animEntry{})
		return Sequence{}
	}
	if key != e.key {
		e.key = key
		e.startTime = u.time
	}
	u.store.set(id, e)
	elapsed := u.time - e.startTime
	if elapsed < 0 {
		elapsed = 0
	}
	return Sequence{elapsed: elapsed, active: true}
}

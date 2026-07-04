package ui

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func testCam() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
}

func TestAnimStoreEvicts(t *testing.T) {
	var s AnimStore
	s.set("a", animEntry{value: 1})

	for range animGCMaxAge + 1 {
		s.AdvanceFrame()
	}

	if _, ok := s.get("a"); ok {
		t.Fatal("entry should be evicted after animGCMaxAge frames without access")
	}
}

func TestAnimStoreSurvivesIfTouched(t *testing.T) {
	var s AnimStore
	s.set("a", animEntry{value: 1})

	// touch on the last frame of the window — entry should survive
	for i := range animGCMaxAge {
		s.AdvanceFrame()
		if i == animGCMaxAge-1 {
			s.set("a", animEntry{value: 1})
		}
	}

	if _, ok := s.get("a"); !ok {
		t.Fatal("entry should survive when touched within the GC window")
	}
}

func TestApproachConverges(t *testing.T) {
	store := AnimStore{}
	var v float64
	for range 300 {
		store.AdvanceFrame()
		u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 1.0/60, 0)
		v = u.Approach("x", 1.0, 12.0)
	}

	if math.Abs(v-1.0) > 1e-4 {
		t.Fatalf("Approach did not converge to target: got %v", v)
	}
}

func TestApproachFrameRateIndependent(t *testing.T) {
	runApproach := func(steps int, dt float64) float64 {
		store := AnimStore{}
		var v float64
		for range steps {
			store.AdvanceFrame()
			u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, dt, 0)
			v = u.Approach("x", 1.0, 12.0)
		}
		return v
	}

	// 1 second at 60 Hz vs 1 second at 30 Hz — same wall time, same result
	at60 := runApproach(60, 1.0/60)
	at30 := runApproach(30, 1.0/30)

	if math.Abs(at60-at30) > 1e-9 {
		t.Fatalf("Approach is frame-rate dependent: 60Hz=%v 30Hz=%v diff=%v", at60, at30, math.Abs(at60-at30))
	}
}

func TestSequenceTrackWindows(t *testing.T) {
	newSeq := func(elapsed float64) Sequence {
		return Sequence{elapsed: elapsed, active: true}
	}

	// before the window: clamped at 0
	tr := newSeq(0).Track(0.5, 1.0, ease.Linear)
	if tr.Value() != 0 {
		t.Fatalf("Track before window = %v, want 0", tr.Value())
	}
	if tr.Done() {
		t.Fatal("Track before window should not be Done")
	}

	// midpoint of window
	tr = newSeq(0.75).Track(0.5, 1.0, ease.Linear)
	if math.Abs(tr.Value()-0.5) > 1e-9 {
		t.Fatalf("Track at midpoint = %v, want 0.5", tr.Value())
	}
	if tr.Done() {
		t.Fatal("Track at midpoint should not be Done")
	}

	// at end: clamped at 1
	tr = newSeq(1.0).Track(0.5, 1.0, ease.Linear)
	if tr.Value() != 1 {
		t.Fatalf("Track at end = %v, want 1", tr.Value())
	}
	if !tr.Done() {
		t.Fatal("Track at end should be Done")
	}

	// past end: still saturated at 1
	tr = newSeq(2.0).Track(0.5, 1.0, ease.Linear)
	if tr.Value() != 1 {
		t.Fatalf("Track past end = %v, want 1", tr.Value())
	}
	if !tr.Done() {
		t.Fatal("Track past end should be Done")
	}

	// Lerp: at midpoint from 0→100 → 50
	tr = newSeq(0.75).Track(0.5, 1.0, ease.Linear)
	if math.Abs(tr.Lerp(0, 100)-50) > 1e-9 {
		t.Fatalf("Track.Lerp at midpoint = %v, want 50", tr.Lerp(0, 100))
	}
}

func TestSequenceKeyedRetriggers(t *testing.T) {
	seqAt := func(store *AnimStore, time float64, key uint64) Sequence {
		store.AdvanceFrame()
		u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, store, 0, time)
		return u.SequenceKeyed("seq", key)
	}

	store := AnimStore{}

	// key=0: never triggered, Done regardless of dur
	seq := seqAt(&store, 0, 0)
	if !seq.Done(1.0) {
		t.Fatal("key=0 should always report Done")
	}

	// key=1 at time=10: sequence starts
	seq = seqAt(&store, 10, 1)
	if seq.Done(1.0) {
		t.Fatal("key=1 just started: should not be Done")
	}
	if seq.elapsed != 0 {
		t.Fatalf("just-started elapsed = %v, want 0", seq.elapsed)
	}

	// same key=1 at time=10.5: elapsed=0.5, not done
	seq = seqAt(&store, 10.5, 1)
	if math.Abs(seq.elapsed-0.5) > 1e-9 {
		t.Fatalf("elapsed = %v, want 0.5", seq.elapsed)
	}

	// same key=1 at time=11.5: elapsed=1.5, Done(1.0)
	seq = seqAt(&store, 11.5, 1)
	if !seq.Done(1.0) {
		t.Fatalf("elapsed=%v should be Done(1.0)", seq.elapsed)
	}

	// key changes to 2 at time=20: sequence restarts
	seq = seqAt(&store, 20, 2)
	if seq.elapsed != 0 {
		t.Fatalf("new key: elapsed = %v, want 0", seq.elapsed)
	}
	if seq.Done(1.0) {
		t.Fatal("new key: sequence just restarted, should not be Done")
	}

	// same key=2 at time=20.3: elapsed=0.3
	seq = seqAt(&store, 20.3, 2)
	if math.Abs(seq.elapsed-0.3) > 1e-9 {
		t.Fatalf("elapsed = %v, want 0.3", seq.elapsed)
	}

	// key=0 clears the retained key, so the same non-zero key can re-trigger
	// after a rewind/render pass where the event is absent.
	seq = seqAt(&store, 21, 0)
	if !seq.Done(1.0) {
		t.Fatal("key=0 after a trigger should report Done")
	}
	seq = seqAt(&store, 22, 2)
	if seq.elapsed != 0 {
		t.Fatalf("same key after key=0: elapsed = %v, want 0", seq.elapsed)
	}
}

func TestSequenceDoneNeverTriggered(t *testing.T) {
	var s Sequence // zero value: never triggered
	if !s.Done(0) {
		t.Fatal("zero Sequence should report Done")
	}
	if !s.Done(100) {
		t.Fatal("zero Sequence should report Done for any duration")
	}
	if s.Track(0, 1, ease.Linear).Value() != 0 {
		t.Fatal("zero Sequence Track should return zero value")
	}
}

func TestSpringDtClamped(t *testing.T) {
	cfg := SpringConfig{Stiffness: 200, Damping: 28}
	store := AnimStore{}
	store.AdvanceFrame()

	// a 500ms hitch must not explode — value should stay finite and within reason
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 0.5, 0)
	v := u.Spring("s", 1.0, cfg)

	if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 100 {
		t.Fatalf("Spring exploded after 500ms dt: %v", v)
	}
}

func TestSpringZeroConfigUsesDefaults(t *testing.T) {
	store := AnimStore{}
	store.AdvanceFrame()

	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 1.0/60, 0)
	v := u.Spring("s", 1.0, SpringConfig{})

	if v <= 0 {
		t.Fatalf("Spring with zero config = %v, want movement toward target using defaults", v)
	}
}

func TestTweenClampsAndResets(t *testing.T) {
	store := AnimStore{}
	var renderTime float64
	step := func(dt float64) float64 {
		store.AdvanceFrame()
		renderTime += dt
		u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, dt, renderTime)
		return u.Tween("t", 1.0, ease.Linear)
	}

	// first call: tween starts, elapsed=0 → returns 0
	step(0)

	// 0.5s later: elapsed=0.5 over 1.0s duration → 0.5
	v := step(0.5)
	if math.Abs(v-0.5) > 1e-9 {
		t.Fatalf("Tween at 0.5s/1.0s = %v, want 0.5", v)
	}

	// past duration: must saturate at 1
	v = step(1.0)
	if v != 1 {
		t.Fatalf("Tween past duration = %v, want 1", v)
	}

	// Reset: next call should restart from 0
	store.AdvanceFrame()
	renderTime += 0.01
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 0.01, renderTime)
	u.Reset("t")

	store.AdvanceFrame()
	renderTime += 0.01
	u = New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 0.01, renderTime)
	v = u.Tween("t", 1.0, ease.Linear)
	if v > 0.1 {
		t.Fatalf("Tween after Reset = %v, want near 0", v)
	}
}

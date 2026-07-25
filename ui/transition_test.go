package ui

import (
	"testing"

	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

func TestFirstScreenDoesNotAnimate(t *testing.T) {
	tr := transitionAt(&AnimStore{}, 10, 1, TransitionOptions{Duration: 0.5})

	if tr.Active() {
		t.Fatal("first screen should not animate")
	}
	if tr.Incoming() != 1 {
		t.Fatalf("incoming = %d, want 1", tr.Incoming())
	}
	if outgoing, ok := tr.Outgoing(); ok || outgoing != 0 {
		t.Fatalf("outgoing = %d, ok=%v, want no outgoing", outgoing, ok)
	}
	if tr.IncomingGroup() != (GroupOptions{}) {
		t.Fatalf("settled incoming group = %+v, want no-op", tr.IncomingGroup())
	}
}

func TestKeyChangeStartsTransition(t *testing.T) {
	var store AnimStore
	transitionAt(&store, 10, 1, TransitionOptions{Duration: 0.5})
	tr := transitionAt(&store, 10.1, 2, TransitionOptions{Duration: 0.5})

	if !tr.Active() {
		t.Fatal("key change should start a transition")
	}
	if tr.Incoming() != 2 {
		t.Fatalf("incoming = %d, want 2", tr.Incoming())
	}
	if outgoing, ok := tr.Outgoing(); !ok || outgoing != 1 {
		t.Fatalf("outgoing = %d, ok=%v, want outgoing 1", outgoing, ok)
	}
}

func TestTransitionCompletesAfterDuration(t *testing.T) {
	var store AnimStore
	transitionAt(&store, 10, 1, TransitionOptions{Duration: 0.5})
	transitionAt(&store, 10.1, 2, TransitionOptions{Duration: 0.5})
	tr := transitionAt(&store, 10.7, 2, TransitionOptions{Duration: 0.5})

	if tr.Active() {
		t.Fatal("transition should be inactive after duration")
	}
	if outgoing, ok := tr.Outgoing(); ok || outgoing != 0 {
		t.Fatalf("outgoing after completion = %d, ok=%v, want none", outgoing, ok)
	}
	if tr.Incoming() != 2 {
		t.Fatalf("incoming after completion = %d, want 2", tr.Incoming())
	}
}

func TestKeyZeroClearsState(t *testing.T) {
	var store AnimStore
	transitionAt(&store, 10, 1, TransitionOptions{Duration: 0.5})
	transitionAt(&store, 10.1, 0, TransitionOptions{Duration: 0.5})
	tr := transitionAt(&store, 10.2, 3, TransitionOptions{Duration: 0.5})

	if tr.Active() {
		t.Fatal("first non-zero key after clear should not animate")
	}
	if tr.Incoming() != 3 {
		t.Fatalf("incoming = %d, want 3", tr.Incoming())
	}
	if outgoing, ok := tr.Outgoing(); ok || outgoing != 0 {
		t.Fatalf("outgoing after clear = %d, ok=%v, want none", outgoing, ok)
	}
}

func TestInterruptRestartsFromCurrent(t *testing.T) {
	var store AnimStore
	transitionAt(&store, 10, 1, TransitionOptions{Duration: 1})
	transitionAt(&store, 10.1, 2, TransitionOptions{Duration: 1})
	tr := transitionAt(&store, 10.2, 3, TransitionOptions{Duration: 1})

	if !tr.Active() {
		t.Fatal("interrupted transition should be active")
	}
	if tr.Incoming() != 3 {
		t.Fatalf("incoming = %d, want 3", tr.Incoming())
	}
	if outgoing, ok := tr.Outgoing(); !ok || outgoing != 2 {
		t.Fatalf("outgoing = %d, ok=%v, want current screen 2", outgoing, ok)
	}
	if tr.IncomingGroup().Fade != 1 {
		t.Fatalf("interrupted transition progress should restart at fade 1, got %+v", tr.IncomingGroup())
	}
}

func TestFadeRecipeAlphas(t *testing.T) {
	var store AnimStore
	transitionAt(&store, 10, 1, TransitionOptions{Kind: TransitionFade, Duration: 1, Ease: ease.Linear})
	start := transitionAt(&store, 10, 2, TransitionOptions{Kind: TransitionFade, Duration: 1, Ease: ease.Linear})
	mid := transitionAt(&store, 10.5, 2, TransitionOptions{Kind: TransitionFade, Duration: 1, Ease: ease.Linear})
	end := transitionAt(&store, 11, 2, TransitionOptions{Kind: TransitionFade, Duration: 1, Ease: ease.Linear})

	if start.IncomingGroup().Fade != 1 || start.OutgoingGroup().Fade != 0 {
		t.Fatalf("fade at start = incoming %+v outgoing %+v, want 1 and 0", start.IncomingGroup(), start.OutgoingGroup())
	}
	if mid.IncomingGroup().Fade != 0.5 || mid.OutgoingGroup().Fade != 0.5 {
		t.Fatalf("fade at midpoint = incoming %+v outgoing %+v, want 0.5 and 0.5", mid.IncomingGroup(), mid.OutgoingGroup())
	}
	if end.IncomingGroup().Fade != 0 || end.OutgoingGroup().Fade != 1 {
		t.Fatalf("fade at end = incoming %+v outgoing %+v, want 0 and 1", end.IncomingGroup(), end.OutgoingGroup())
	}
}

func TestSlideLeftRecipeOffsets(t *testing.T) {
	var store AnimStore
	opts := TransitionOptions{Kind: TransitionSlideLeft, Duration: 1, Ease: ease.Linear}
	transitionAt(&store, 10, 1, opts)
	start := transitionAt(&store, 10, 2, opts)
	mid := transitionAt(&store, 10.5, 2, opts)
	end := transitionAt(&store, 11, 2, opts)

	if start.IncomingGroup().Offset != (math2d.Vec2{X: 100}) || start.OutgoingGroup().Offset != (math2d.Vec2{}) {
		t.Fatalf("slide start = incoming %+v outgoing %+v, want +W and 0", start.IncomingGroup(), start.OutgoingGroup())
	}
	if mid.IncomingGroup().Offset != (math2d.Vec2{X: 50}) || mid.OutgoingGroup().Offset != (math2d.Vec2{X: -50}) {
		t.Fatalf("slide midpoint = incoming %+v outgoing %+v, want +W/2 and -W/2", mid.IncomingGroup(), mid.OutgoingGroup())
	}
	if end.IncomingGroup().Offset != (math2d.Vec2{}) || end.OutgoingGroup().Offset != (math2d.Vec2{X: -100}) {
		t.Fatalf("slide end = incoming %+v outgoing %+v, want 0 and -W", end.IncomingGroup(), end.OutgoingGroup())
	}
}

func TestUpdateFaceReturnsSettled(t *testing.T) {
	u := NewUpdate(&State{}, zeroPointerFrame(), testCam(), DefaultTheme(), testCam().Viewport)
	tr := u.ScreenTransition("screen", 7, TransitionOptions{Duration: 0.5})

	if tr.Active() {
		t.Fatal("update face without AnimStore should return a settled transition")
	}
	if tr.Incoming() != 7 {
		t.Fatalf("incoming = %d, want 7", tr.Incoming())
	}
	if outgoing, ok := tr.Outgoing(); ok || outgoing != 0 {
		t.Fatalf("outgoing = %d, ok=%v, want none", outgoing, ok)
	}
}

func transitionAt(store *AnimStore, renderTime float64, key uint64, opts TransitionOptions) ScreenTransition {
	store.AdvanceFrame()
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, store, 0, renderTime)
	return u.ScreenTransition("screen", key, opts)
}

func zeroPointerFrame() input.PointerFrame {
	return input.PointerFrame{}
}

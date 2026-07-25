package ui

import (
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
)

const defaultTransitionDuration = 0.35

// TransitionKind selects the ready-made motion recipe for a screen change.
type TransitionKind uint8

const (
	// TransitionFade cross-fades: outgoing fades out, incoming fades in.
	TransitionFade TransitionKind = iota
	// TransitionSlideLeft slides the outgoing screen out to the left while
	// the incoming screen enters from the right.
	TransitionSlideLeft
	// TransitionSlideRight mirrors TransitionSlideLeft.
	TransitionSlideRight
)

// TransitionOptions configures ScreenTransition.
type TransitionOptions struct {
	Kind     TransitionKind
	Duration float64
	Ease     ease.Kind
}

// ScreenTransition reports the state of a keyed screen change and hands out
// the GroupOptions for drawing each side. Render-only: it never affects the
// simulation, rewind snapshots, or the checksum.
type ScreenTransition struct {
	from, to uint64
	t        float64 // eased progress in [0,1]
	active   bool
	kind     TransitionKind
	refSize  math2d.Vec2
}

func (u *UI) ScreenTransition(id string, key uint64, opts TransitionOptions) ScreenTransition {
	if u.store == nil {
		return ScreenTransition{to: key, t: 1, kind: opts.Kind}
	}
	if opts.Duration <= 0 {
		opts.Duration = defaultTransitionDuration
	}

	e, seen := u.store.get(id)
	if key == 0 {
		u.store.set(id, animEntry{})
		return ScreenTransition{kind: opts.Kind}
	}
	switch {
	case !seen || e.key == 0:
		// First screen: adopt without animating (startTime in the past).
		e.key = key
		e.prevKey = 0
		e.startTime = u.time - opts.Duration
	case key != e.key:
		e.prevKey = e.key
		e.key = key
		e.startTime = u.time
	}
	u.store.set(id, e)

	raw := ease.Progress(u.time-e.startTime, opts.Duration)
	tr := ScreenTransition{
		from:    e.prevKey,
		to:      e.key,
		t:       ease.Apply(opts.Ease, raw),
		active:  raw < 1 && e.prevKey != 0,
		kind:    opts.Kind,
		refSize: u.reference.Size(),
	}
	return tr
}

// Active reports whether a transition is currently animating.
func (tr ScreenTransition) Active() bool { return tr.active }

// Incoming returns the key of the current (entering or settled) screen.
func (tr ScreenTransition) Incoming() uint64 { return tr.to }

// Outgoing returns the key of the exiting screen. ok is false when no
// transition is running — skip drawing the outgoing screen then.
func (tr ScreenTransition) Outgoing() (key uint64, ok bool) {
	if !tr.active {
		return 0, false
	}
	return tr.from, true
}

// IncomingGroup returns the GroupOptions to wrap the incoming screen's
// drawing with. When no transition is running it is a no-op group.
func (tr ScreenTransition) IncomingGroup() GroupOptions {
	if !tr.active {
		return GroupOptions{}
	}
	switch tr.kind {
	case TransitionSlideLeft:
		return GroupOptions{Offset: math2d.Vec2{X: tr.refSize.X * (1 - tr.t)}}
	case TransitionSlideRight:
		return GroupOptions{Offset: math2d.Vec2{X: -tr.refSize.X * (1 - tr.t)}}
	default: // TransitionFade
		return GroupOptions{Fade: 1 - tr.t}
	}
}

// OutgoingGroup returns the GroupOptions for the exiting screen.
func (tr ScreenTransition) OutgoingGroup() GroupOptions {
	switch tr.kind {
	case TransitionSlideLeft:
		return GroupOptions{Offset: math2d.Vec2{X: -tr.refSize.X * tr.t}}
	case TransitionSlideRight:
		return GroupOptions{Offset: math2d.Vec2{X: tr.refSize.X * tr.t}}
	default:
		return GroupOptions{Fade: tr.t}
	}
}

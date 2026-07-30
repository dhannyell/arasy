package runtime

import (
	"time"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// splashContent renders a texture-free segment at timeline time. The returned
// commands are consumed synchronously and may be backed by a reusable
// workspace. Content must be a pure function of (timeline, viewport): it runs
// before tick 0 and never observes world/game state.
type splashContent func(
	timeline anim.Timeline,
	viewport math2d.Rect,
) []render.Command

// splashSegment is one entry in the boot splash sequence, shown for at least
// MinTime, optionally cross-faded and skippable. A segment renders either a
// texture flip-book (Texture set — a static segment sets Frames <= 1) or a
// vector command Content (e.g. the compiled engine brand animation); Content
// takes precedence when both are set. Content is render-only and lives
// entirely before tick 0.
//
// Games declare segments through the asset manifest's "splash" array, never
// by constructing this type; it is unexported because no caller outside the
// runtime builds playlists. Future content kinds belong in the manifest.
type splashSegment struct {
	Texture render.TextureID // atlas/image to show

	// Grid describes the flip-book atlas. CellW/CellH are the cell size in
	// texture pixels; Columns is cells per row. Frames <= 1 draws Cell(0)
	// statically (Columns/Cell still locate the cell).
	CellW, CellH float64
	Columns      int
	Frames       uint16
	Rate         float32 // flip-book frames per second

	// Content, when non-nil, renders this segment as a vector command
	// timeline instead of the texture flip-book above. No manifest entry or
	// texture upload is needed for this content.
	Content splashContent

	MinTime float64 // minimum on-screen seconds
	Fade    float64 // cross-fade seconds at each edge; 0 = hard cut
	// Skippable lets an activation event (input.EventButtonDown) end the
	// segment before MinTime; passive input never skips it.
	Skippable bool

	// Background paints behind this segment's content, covering the full
	// viewport (never faded). Zero value uses Config.BootBackground, then
	// Config.Clear. Content whose own asset already paints a flat backdrop
	// only covering its own letterboxed area (like the compiled engine
	// animation) should set this to the same color, or the two backgrounds
	// seam together whenever the viewport's aspect ratio isn't the content's
	// own.
	Background render.Color

	// Particles, when non-nil, computes this segment's particle emissions
	// (zero or more) for its own local elapsed time and per-frame delta.
	// render.Builder rejects DrawParticles under any modified (non-identity)
	// group, so this bypasses the Content fade group entirely — a segment
	// that wants its particles to fade with the rest of its content must
	// fold alpha into each returned command's Intensity itself. A nil or
	// empty result draws nothing this frame.
	Particles func(elapsed, frameDt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles

	source splashSource
}

type splashSource uint8

const (
	splashSourceGame splashSource = iota
	splashSourceDefault
)

// splashPlayer drives the boot splash sequence. It never touches d.tick, the
// timeline, the recorder, or the checksum. Zero segments = inert.
type splashPlayer struct {
	segments []splashSegment
	index    int
	elapsed  float64
}

// SplashFrameBatch is the host state consumed by one real-time splash frame.
// Events are discarded by the host when RunSplashFrame reports Rendered.
// Only EventButtonDown is an activation request that can skip a skippable
// segment; pointer motion, releases, wheel input and other passive events do
// not skip it.
type SplashFrameBatch struct {
	Viewport *math2d.Rect
	Events   []input.Event
}

// SplashFrameResult tells a host whether RunSplashFrame consumed and rendered
// the current host frame, and whether that rendered frame can replace a
// host-owned loading poster without exposing partially faded content.
type SplashFrameResult struct {
	Rendered        bool
	ReadyForHandoff bool
}

// SkipSplash removes the remaining runtime boot splash playlist. Web hosts
// use it before presenting the first frame when their command-line session
// explicitly disables runtime splash screens; the independent HTML download
// indicator and simulation tick state are untouched.
func (d *Driver) SkipSplash() {
	d.splash = nil
}

// SkipDefaultSplash removes only the built-in engine brand segment, preserving
// every game-declared manifest segment. Browser hosts call it before their
// first frame only for an explicit host-level engine-brand opt-out; otherwise
// Config.DisableDefaultSplash remains authoritative on every host. Simulation
// tick state is untouched.
func (d *Driver) SkipDefaultSplash() {
	if d.splash == nil {
		return
	}

	currentWasDefault := d.splashActive() &&
		d.splash.segments[d.splash.index].source == splashSourceDefault
	removedBeforeCurrent := 0
	segments := make([]splashSegment, 0, len(d.splash.segments))
	for i, segment := range d.splash.segments {
		if segment.source == splashSourceDefault {
			if i < d.splash.index {
				removedBeforeCurrent++
			}
			continue
		}
		segments = append(segments, segment)
	}

	d.splash.segments = segments
	d.splash.index -= removedBeforeCurrent
	if currentWasDefault {
		d.splash.elapsed = 0
	}
	if d.splash.index >= len(segments) {
		d.splash = nil
	}
}

func (d *Driver) splashActive() bool {
	return d.splash != nil && d.splash.index < len(d.splash.segments)
}

func (d *Driver) splashFrameReadyForHandoff() bool {
	if !d.splashActive() {
		return false
	}
	seg := d.splash.segments[d.splash.index]
	return splashFadeAlpha(d.splash.elapsed, seg.MinTime, seg.Fade) >= 1
}

// RunSplashFrame owns the canonical pre-tick-0 host sequence: apply the
// viewport, interpret activation input, advance the current segment, render
// it, and determine poster handoff readiness. frameDelta is the host frame's
// wall-clock delta — not the segment's accumulated time. A zero result means
// no splash was active; the host must retain the input and continue with its
// game frame. A rendered splash consumes all supplied events but never
// advances d.tick or invokes game state. Viewport changes retain SetViewport's
// normal tick-0 timeline semantics.
func (d *Driver) RunSplashFrame(batch SplashFrameBatch, frameDelta time.Duration, r FrameRenderer) SplashFrameResult {
	if !d.splashActive() {
		return SplashFrameResult{}
	}
	if batch.Viewport != nil {
		d.SetViewport(*batch.Viewport)
	}

	skip := false
	for _, event := range batch.Events {
		if event.Kind == input.EventButtonDown {
			skip = true
			break
		}
	}

	d.advanceSplash(frameDelta, skip)
	d.renderSplashFrame(frameDelta, r)
	return SplashFrameResult{
		Rendered:        true,
		ReadyForHandoff: d.splashFrameReadyForHandoff(),
	}
}

func (d *Driver) advanceSplash(frameDelta time.Duration, skip bool) {
	if !d.splashActive() {
		return
	}
	seg := d.splash.segments[d.splash.index]
	d.splash.elapsed += frameDelta.Seconds()
	done := d.splash.elapsed >= seg.MinTime || (skip && seg.Skippable)
	if done {
		d.splash.index++
		d.splash.elapsed = 0
	}
}

func (d *Driver) renderSplashFrame(frameDelta time.Duration, r FrameRenderer) {
	frame := d.splashFrame(frameDelta.Seconds())
	r.UpdateResources(FrameUpdates{})
	r.Render(frame)
}

package runtime

import (
	"math"
	"time"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/enginesplash"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// SplashContent renders a texture-free segment at timeline time. The returned
// commands are consumed synchronously and may be backed by a reusable
// workspace. Content must be a pure function of (timeline, viewport): it runs
// before tick 0 and never observes world/game state.
type SplashContent func(
	timeline anim.Timeline,
	viewport math2d.Rect,
) []render.Command

// SplashSegment is one entry in the boot splash sequence, shown for at least
// MinTime, optionally cross-faded and skippable. A segment renders either a
// texture flip-book (Texture set — a static segment sets Frames <= 1) or a
// vector command Content (e.g. the compiled engine brand animation); Content
// takes precedence when both are set. Content is render-only and lives
// entirely before tick 0.
type SplashSegment struct {
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
	Content SplashContent

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
	Particles func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles

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
	segments []SplashSegment
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
	segments := make([]SplashSegment, 0, len(d.splash.segments))
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
// it, and determine poster handoff readiness. A zero result means no splash
// was active; the host must retain the input and continue with its game frame.
// A rendered splash consumes all supplied events but never advances d.tick or
// invokes game state. Viewport changes retain SetViewport's normal tick-0
// timeline semantics.
func (d *Driver) RunSplashFrame(batch SplashFrameBatch, elapsed time.Duration, r FrameRenderer) SplashFrameResult {
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

	d.advanceSplash(elapsed, skip)
	d.renderSplashFrame(elapsed, r)
	return SplashFrameResult{
		Rendered:        true,
		ReadyForHandoff: d.splashFrameReadyForHandoff(),
	}
}

func (d *Driver) advanceSplash(elapsed time.Duration, skip bool) {
	if !d.splashActive() {
		return
	}
	seg := d.splash.segments[d.splash.index]
	d.splash.elapsed += elapsed.Seconds()
	done := d.splash.elapsed >= seg.MinTime || (skip && seg.Skippable)
	if done {
		d.splash.index++
		d.splash.elapsed = 0
	}
}

func (d *Driver) renderSplashFrame(elapsed time.Duration, r FrameRenderer) {
	frame := d.splashFrame(elapsed.Seconds())
	r.UpdateResources(FrameUpdates{})
	r.Render(frame)
}

func (d *Driver) splashFrame(dt float64) Frame {
	cam := render.Camera{World: d.viewport, Viewport: d.viewport}
	if !d.splashActive() {
		// The frame that drains the playlist is still presented by hosts (it
		// consumes the final skip/timeout without leaking that input into
		// tick 0). Paint the last segment's backdrop instead of returning an
		// empty frame, so the transition to the first game frame never
		// flashes a bare Config.Clear screen.
		if d.splash == nil || len(d.splash.segments) == 0 {
			return Frame{Camera: cam}
		}
		last := d.splash.segments[len(d.splash.segments)-1]
		return Frame{
			Camera:   cam,
			Commands: []render.Command{d.splashBackgroundRect(last)},
		}
	}

	seg := d.splash.segments[d.splash.index]
	alpha := splashFadeAlpha(d.splash.elapsed, seg.MinTime, seg.Fade)

	commands := []render.Command{d.splashBackgroundRect(seg)}

	var builder render.Builder
	builder.Reset(commands)
	builder.Push(render.Group{Opacity: alpha})
	if seg.Content != nil {
		content := seg.Content(anim.At(d.splash.elapsed), d.viewport)
		for _, cmd := range content {
			builder.Add(cmd)
		}
	} else {
		builder.Add(splashFlipbookImage(seg, d.splash.elapsed, d.viewport))
	}
	builder.Pop()

	commands = builder.Commands()
	if seg.Particles != nil {
		for _, p := range seg.Particles(d.splash.elapsed, dt, d.viewport, alpha) {
			commands = append(commands, p)
		}
	}

	return Frame{Camera: cam, Commands: commands}
}

// splashBackgroundRect paints the backdrop behind a segment across the full
// viewport: the segment's own Background, then Config.BootBackground, then
// Config.Clear.
func (d *Driver) splashBackgroundRect(seg SplashSegment) render.DrawRect {
	background := seg.Background
	if background == (render.Color{}) {
		background = d.cfg.BootBackground
		if background == (render.Color{}) {
			background = d.cfg.Clear
		}
	}
	return render.DrawRect{
		BaseCommand: render.BaseCommand{
			LayerValue: enginesplash.BackgroundLayer,
		},
		Rect: d.viewport,
		Fill: background,
	}
}

// splashFlipbookImage computes the current cell of a texture flip-book
// segment and its letterboxed destination rectangle. The cell advance
// mirrors internal/renderbatch's flip-book arithmetic, but is driven by the
// segment's own Columns (the Driver never decodes texture dimensions).
func splashFlipbookImage(seg SplashSegment, elapsed float64, viewport math2d.Rect) render.DrawImage {
	frame := 0
	if seg.Frames > 1 && seg.Rate > 0 {
		tw := float64(renderbatch.WrapTime(elapsed))
		frame = int(math.Floor(tw*float64(seg.Rate))) % int(seg.Frames)
	}

	col := frame % seg.Columns
	row := frame / seg.Columns
	src := math2d.NewRect(
		math2d.Vec2{X: float64(col) * seg.CellW, Y: float64(row) * seg.CellH},
		math2d.Vec2{X: seg.CellW, Y: seg.CellH},
	)
	dest := fitCentered(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: seg.CellW, Y: seg.CellH}), viewport)

	return render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     seg.Texture,
		Dest:        dest,
		Src:         src,
	}
}

// splashFadeAlpha computes the cross-fade envelope for a segment: it rises
// over Fade seconds at the start and falls over Fade seconds before MinTime
// ends. Fade <= 0 is a hard cut (always opaque).
//
// A host with its own loading screen (e.g. internal/wasmserve's poster)
// must hide it with a hard cut, not its own opacity transition — stacking
// a second fade on top of this rise makes the reveal visibly dip instead of
// resolving to one smooth fade-in.
func splashFadeAlpha(elapsed, minTime, fade float64) float64 {
	if fade <= 0 {
		return 1
	}
	alpha := 1.0
	if elapsed < fade {
		alpha = elapsed / fade
	}
	if remain := minTime - elapsed; remain < fade {
		if out := remain / fade; out < alpha {
			alpha = out
		}
	}
	return math2d.Clamp(alpha, 0, 1)
}

// fitCentered scales cell to fit inside viewport preserving aspect ratio
// (letterbox), centered on both axes.
func fitCentered(cell, viewport math2d.Rect) math2d.Rect {
	cellW, cellH := cell.Width(), cell.Height()
	vw, vh := viewport.Width(), viewport.Height()
	if cellW <= 0 || cellH <= 0 || vw <= 0 || vh <= 0 {
		return viewport
	}

	scale := math.Min(vw/cellW, vh/cellH)
	fitW, fitH := cellW*scale, cellH*scale
	offset := math2d.Vec2{
		X: viewport.Min.X + (vw-fitW)/2,
		Y: viewport.Min.Y + (vh-fitH)/2,
	}
	return math2d.NewRect(offset, math2d.Vec2{X: fitW, Y: fitH})
}

// buildSplashPlaylist assembles the boot splash playlist: the built-in
// engine brand segment (unless disabled), followed by the manifest-declared
// segments in order.
func buildSplashPlaylist(cfg Config) []SplashSegment {
	var segments []SplashSegment
	if !cfg.DisableDefaultSplash {
		var content enginesplash.Content
		segments = append(segments, SplashSegment{
			Content:    content.Append,
			MinTime:    enginesplash.MinTime,
			Fade:       enginesplash.Fade,
			Skippable:  enginesplash.Skippable,
			Background: enginesplash.Background,
			Particles:  enginesplash.NewParticles(cfg.Assets),
			source:     splashSourceDefault,
		})
	}
	for index, decl := range cfg.Assets.Splash() {
		segments = append(segments, SplashSegment{
			Texture:   cfg.Assets.Texture(decl.Texture),
			CellW:     decl.CellW,
			CellH:     decl.CellH,
			Columns:   decl.Columns,
			Frames:    decl.Frames,
			Rate:      decl.Rate,
			MinTime:   decl.MinTime,
			Fade:      decl.Fade,
			Skippable: decl.Skippable,
			Particles: manifestSplashParticles(cfg.Assets, index, decl),
		})
	}
	return segments
}

// manifestSplashParticles resolves a game-declared segment's optional
// "effect" reference into a segment-scoped emitter at a fractional viewport
// anchor.
func manifestSplashParticles(assets asset.Manifest, segmentIndex int, decl asset.SplashDecl) func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	if decl.Effect == "" {
		return nil
	}
	effect := assets.Effect(decl.Effect)
	// EmitterSet keys live state by (effect, emitter) and only rebuilds an
	// existing key when its seed changes. Scoping both values to manifest
	// order prevents consecutive segments that share an effect from
	// inheriting each other's particles while keeping each segment stable
	// across its own frames.
	emitter := render.EmitterID(segmentIndex + 1)
	seed := uint64(effect)<<32 | uint64(emitter)
	intensity := decl.EffectIntensity
	if intensity == 0 {
		intensity = 1
	}
	return func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
		if effect == 0 || alpha <= 0 {
			return nil
		}
		pos := math2d.Vec2{
			X: viewport.Min.X + viewport.Width()*decl.EffectAnchorX,
			Y: viewport.Min.Y + viewport.Height()*decl.EffectAnchorY,
		}
		return []render.DrawParticles{{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Effect:      effect,
			Emitter:     emitter,
			Pos:         pos,
			Intensity:   intensity * alpha,
			Time:        elapsed,
			Dt:          dt,
			Seed:        seed,
		}}
	}
}

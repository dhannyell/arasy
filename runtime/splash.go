package runtime

import (
	"math"
	"time"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/enginesplash"
	"github.com/dhannyell/arasy/internal/enginesplash/stacked"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// SplashContent renders a texture-free segment at timeline time into dst,
// returning the extended slice. It mirrors the Append<Name> signature the
// arasy svg compiler generates, so a compiled brand animation can be used
// directly as a segment's Content with resources bound by closure. Content
// must be a pure function of (timeline, viewport): it runs before tick 0 and
// never observes world/game state.
type SplashContent func(
	dst []render.Command,
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

	MinTime   float64 // minimum on-screen seconds
	Fade      float64 // cross-fade seconds at each edge; 0 = hard cut
	Skippable bool

	// Background paints behind this segment's content, covering the full
	// viewport (never faded). Zero value uses Config.Clear. Content whose
	// own asset already paints a flat backdrop only covering its own
	// letterboxed area (like the compiled engine animation) should set this
	// to the same color, or the two backgrounds seam together whenever the
	// viewport's aspect ratio isn't the content's own.
	Background render.Color

	// Particles, when non-nil, computes this segment's particle emissions
	// (zero or more) for its own local elapsed time and per-frame delta.
	// render.Builder rejects DrawParticles under any modified (non-identity)
	// group, so this bypasses the Content fade group entirely — a segment
	// that wants its particles to fade with the rest of its content must
	// fold alpha into each returned command's Intensity itself. A nil or
	// empty result draws nothing this frame.
	Particles func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles
}

// splashPlayer drives the boot splash sequence. It never touches d.tick, the
// timeline, the recorder, or the checksum. Zero segments = inert.
type splashPlayer struct {
	segments []SplashSegment
	index    int
	elapsed  float64
}

// SkipSplash removes the remaining boot splash playlist. Development hosts
// use it before presenting the first frame when their command-line session
// explicitly disables splash screens; simulation tick state is untouched.
func (d *Driver) SkipSplash() {
	d.splash = nil
}

// SplashActive reports whether the boot splash still has a segment to play.
// Only real-time hosts drain it, before their tick loop; offline and replay
// paths never call it, so they reach tick 0 immediately.
func (d *Driver) SplashActive() bool {
	return d.splash != nil && d.splash.index < len(d.splash.segments)
}

// AdvanceSplash advances the current segment by wall-clock elapsed. skip is
// true when any host input arrived this frame; it only ends the segment when
// that segment is Skippable (per its MinTime/Skippable rule). It never
// advances d.tick.
func (d *Driver) AdvanceSplash(elapsed time.Duration, skip bool) {
	if !d.SplashActive() {
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

// RenderSplashFrame draws the current splash segment through r (the same
// FrameRenderer used for game frames). It publishes no dynamic resources.
func (d *Driver) RenderSplashFrame(elapsed time.Duration, r FrameRenderer) Frame {
	frame := d.splashFrame(elapsed.Seconds())
	r.UpdateResources(FrameUpdates{})
	r.Render(frame)
	return frame
}

func (d *Driver) splashFrame(dt float64) Frame {
	cam := render.Camera{World: d.viewport, Viewport: d.viewport}
	if !d.SplashActive() {
		return Frame{Camera: cam}
	}

	seg := d.splash.segments[d.splash.index]
	alpha := splashFadeAlpha(d.splash.elapsed, seg.MinTime, seg.Fade)

	background := seg.Background
	if background == (render.Color{}) {
		background = d.cfg.Clear
	}

	commands := []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{
				LayerValue: enginesplash.EngineSplashBackgroundLayer,
			},
			Rect: d.viewport,
			Fill: background,
		},
	}

	var builder render.Builder
	builder.Reset(commands)
	builder.Push(render.Group{Opacity: alpha})
	if seg.Content != nil {
		content := seg.Content(nil, anim.At(d.splash.elapsed), d.viewport)
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
		segments = append(segments, SplashSegment{
			Content: func(dst []render.Command, timeline anim.Timeline, viewport math2d.Rect) []render.Command {
				if viewport.Width() < viewport.Height() {
					viewport = enginesplash.StackedViewport(viewport)
					return stacked.AppendArasySplashStacked(dst, timeline, viewport, stacked.ArasySplashStackedResources{})
				}
				viewport = enginesplash.EngineSplashViewport(viewport)
				return enginesplash.AppendArasySplash(dst, timeline, viewport, enginesplash.ArasySplashResources{})
			},
			MinTime:    enginesplash.EngineSplashMinTime,
			Fade:       enginesplash.EngineSplashFade,
			Skippable:  false,
			Background: enginesplash.EngineSplashBackground,
			Particles:  enginesplash.EngineSplashParticles(cfg.Assets),
		})
	}
	for _, decl := range cfg.Assets.Splash() {
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
			Particles: manifestSplashParticles(cfg.Assets, decl),
		})
	}
	return segments
}

// manifestSplashParticles resolves a game-declared segment's optional
// "effect" reference into a fixed emitter at a fractional viewport anchor.
func manifestSplashParticles(assets asset.Manifest, decl asset.SplashDecl) func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	if decl.Effect == "" {
		return nil
	}
	effect := assets.Effect(decl.Effect)
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
			Emitter:     1,
			Pos:         pos,
			Intensity:   intensity * alpha,
			Time:        elapsed,
			Dt:          dt,
			Seed:        uint64(effect),
		}}
	}
}

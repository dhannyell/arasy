package runtime

import (
	"math"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/internal/enginesplash"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// splashFrame assembles the current segment's frame. frameDt is the host
// frame's wall-clock delta in seconds (fed to particle emitters); the
// segment's own accumulated time lives in d.splash.elapsed.
func (d *Driver) splashFrame(frameDt float64) Frame {
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
		for _, p := range seg.Particles(d.splash.elapsed, frameDt, d.viewport, alpha) {
			commands = append(commands, p)
		}
	}

	return Frame{Camera: cam, Commands: commands}
}

// splashBackgroundRect paints the backdrop behind a segment across the full
// viewport: the segment's own Background, then Config.BootBackground, then
// Config.Clear.
func (d *Driver) splashBackgroundRect(seg splashSegment) render.DrawRect {
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
func splashFlipbookImage(seg splashSegment, elapsed float64, viewport math2d.Rect) render.DrawImage {
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

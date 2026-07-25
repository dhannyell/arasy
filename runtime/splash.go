package runtime

import (
	"math"
	"time"

	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// SplashSegment is one entry in the boot splash sequence: a sprite shown for
// at least MinTime, optionally cross-faded and skippable. A static segment
// sets Frames <= 1; an animated segment is a flip-book over a grid atlas,
// advanced by the render clock (mirrors internal/renderbatch/animation.go).
// Content is render-only and lives entirely before tick 0.
type SplashSegment struct {
	Texture render.TextureID // atlas/image to show

	// Grid describes the flip-book atlas.

	//CellW/CellH are the cell size in texture pixels;
	CellW, CellH float64
	// Columns are cells per row. Frames <= 1 draws Cell(0)
	// statically (Columns/Cell still locate the cell).
	Columns   int
	Frames    uint16
	Rate      float32 // flip-book frames per second
	MinTime   float64 // minimum on-screen seconds
	Fade      float64 // cross-fade seconds at each edge; 0 = hard cut
	Skippable bool
}

// SplashPlayer drives the boot splash sequence. It never touches d.tick, the
// timeline, the recorder, or the checksum. Zero segments = inert.
type splashPlayer struct {
	segments []SplashSegment
	index    int
	elapsed  float64
}

// SplashActive reports whether the boot splash still has a segment to play.
// Only real-time hosts drain it, before their tick loop; offline and replay
// paths never call it, so they reach tick 0 immediately.
func (d *Driver) SplashActive() bool {
	return d.splash.index < len(d.splash.segments)
}

// AdvanceSplash advances the current segment by wall-clock elapsed. skip is
// true when any host input arrived this frame; it only ends the segment when
// that segment is Skippable (D3). It never advances d.tick.
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

func (d *Driver) RenderSplashFrame(elapsed time.Duration, r FrameRenderer) Frame {
	frame := d.splashFrame()
	r.UpdateResources(FrameUpdates{})
	r.Render(frame)
	return frame
}

func (d *Driver) splashFrame() Frame {
	frame := 0
	for _, seg := range d.splash.segments {
		if seg.Frames > 1 && seg.Rate > 0 {
			tw := float64(renderbatch.WrapTime(d.splash.elapsed))
			frame = int(math.Floor(tw*float64(seg.Rate))) % int(seg.Frames)
		}

		idx := frame
		col := idx % seg.Columns
		row := idx / seg.Columns
		_ = math2d.NewRect(
			math2d.Vec2{X: float64(col) * seg.CellW, Y: float64(row) * seg.CellH},
			math2d.Vec2{X: seg.CellW, Y: seg.CellH},
		)
	}

	return Frame{
		Camera: render.Camera{
			World:    d.viewport,
			Viewport: d.viewport,
		},
		Commands: []render.Command{
			render.DrawImage{
				BaseCommand: render.BaseCommand{
					LayerValue: 1,
				},
				Tint: render.Color{R: 1, G: 1, B: 1, A: 1},
			},
		},
	}
}

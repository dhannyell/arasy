package raster_test

import (
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

type frameSequenceGame struct{}

func (frameSequenceGame) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: 4, Y: 4},
			),
			Viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: 4, Y: 4},
			),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}
}

func (frameSequenceGame) Init(ctx *runtime.UpdateContext) {}

func (frameSequenceGame) Update(ctx *runtime.UpdateContext, dt float64) {}

func (frameSequenceGame) Render(ctx *runtime.RenderContext) []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 1, Y: 1},
				math2d.Vec2{X: 2, Y: 2},
			),
			Fill: render.RGBA(1, 0, 0, 1),
		},
	}
}

func TestRunWritesPNGFrameSequence(t *testing.T) {
	framesDir := filepath.Join(t.TempDir(), "frames")

	err := raster.Run(frameSequenceGame{}, raster.Options{
		Width:     4,
		Height:    4,
		Frames:    3,
		FrameDT:   1.0 / 30.0,
		FramesDir: framesDir,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for i, name := range []string{"frame-0000.png", "frame-0001.png", "frame-0002.png"} {
		path := filepath.Join(framesDir, name)
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open frame %d: %v", i, err)
		}
		if _, err := png.Decode(f); err != nil {
			_ = f.Close()
			t.Fatalf("decode frame %d: %v", i, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close frame %d: %v", i, err)
		}
	}

	if _, err := os.Stat(filepath.Join(framesDir, "frame-0003.png")); !os.IsNotExist(err) {
		t.Fatalf("frame-0003.png exists or stat failed with %v, want not exist", err)
	}
}

var _ runtime.Game = frameSequenceGame{}

// centerDotGame configures a small 4x4 world/viewport and draws a tiny rect
// centered at the world's midpoint — used to detect whether Run maps world
// coordinates using the actual output image size or the game's declared
// (and here intentionally smaller) viewport.
type centerDotGame struct{}

func (centerDotGame) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}
}

func (centerDotGame) Init(ctx *runtime.UpdateContext) {}

func (centerDotGame) Update(ctx *runtime.UpdateContext, dt float64) {}

func (centerDotGame) Render(ctx *runtime.RenderContext) []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 1.9, Y: 1.9},
				math2d.Vec2{X: 0.2, Y: 0.2},
			),
			Fill: render.RGBA(1, 0, 0, 1),
		},
	}
}

var _ runtime.Game = centerDotGame{}

func TestRunMapsCommandsToOutputImageSize(t *testing.T) {
	pngPath := filepath.Join(t.TempDir(), "center.png")

	err := raster.Run(centerDotGame{}, raster.Options{
		Width:  40,
		Height: 40,
		PNG:    pngPath,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatalf("open png: %v", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	r, g, b, _ := img.At(20, 20).RGBA()
	if r>>8 < 200 || g>>8 > 50 || b>>8 > 50 {
		t.Fatalf("pixel at output center (20,20) = rgb(%d,%d,%d), want red — commands were mapped using the "+
			"declared 4x4 viewport instead of the actual 40x40 output image",
			r>>8, g>>8, b>>8)
	}
}

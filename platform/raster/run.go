package raster

import (
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/dhannyell/frameforge/runtime"
)

// Options configures an offline raster render run.
type Options struct {
	Width     int
	Height    int
	Frames    int     // number of simulation frames to render for animated outputs
	FrameDT   float64 // seconds per frame
	PNG       string  // output path for a single PNG of the initial frame; empty = skip
	GIF       string  // output path for an animated GIF; empty = skip
	FramesDir string  // output directory for frame-0000.png sequence; empty = skip
	Log       *slog.Logger
}

// Run drives game through an offline raster render and writes PNG and/or GIF output.
// It mirrors canvas.Run: game is written once; this is the headless/offline target.
func Run(game runtime.Game, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	driver := runtime.NewDriver(game, log)
	cfg := driver.Config()

	r := Renderer{
		Width:  opts.Width,
		Height: opts.Height,
		Camera: cfg.Camera,
		Clear:  cfg.Clear,
	}

	if opts.PNG != "" {
		img := r.Render(driver.Frame().Commands)
		if err := writeFile(opts.PNG, func(f *os.File) error {
			return png.Encode(f, img)
		}); err != nil {
			return err
		}
	}

	if (opts.GIF != "" || opts.FramesDir != "") && opts.Frames > 0 {
		delay := int(math.Round(opts.FrameDT * 100))
		if delay < 1 {
			delay = 1
		}
		var anim *gif.GIF
		if opts.GIF != "" {
			anim = &gif.GIF{
				Image: make([]*image.Paletted, 0, opts.Frames),
				Delay: make([]int, 0, opts.Frames),
			}
		}
		for frame := range opts.Frames {
			img := r.Render(driver.Frame().Commands)
			if opts.FramesDir != "" {
				if err := writeFramePNG(opts.FramesDir, frame, img); err != nil {
					return err
				}
			}
			if anim != nil {
				anim.Image = append(anim.Image, toPaletted(img))
				anim.Delay = append(anim.Delay, delay)
			}
			driver.AdvanceTime(time.Duration(opts.FrameDT * float64(time.Second)))
		}
		if anim != nil {
			if err := writeFile(opts.GIF, func(f *os.File) error {
				return gif.EncodeAll(f, anim)
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeFramePNG(dir string, frame int, img image.Image) error {
	path := filepath.Join(dir, fmt.Sprintf("frame-%04d.png", frame))
	return writeFile(path, func(f *os.File) error {
		return png.Encode(f, img)
	})
}

func writeFile(path string, fn func(*os.File) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f)
}

func toPaletted(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	p := image.NewPaletted(bounds, palette.Plan9)
	draw.FloydSteinberg.Draw(p, bounds, img, image.Point{})
	return p
}

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

	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/math2d"
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

// Run drives a game through the offline raster target.
func Run(game runtime.Game, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	driver := runtime.NewDriver(game, log)
	cfg := driver.Config()

	// A texture that fails to decode is left nil in the registry (rendered as
	// the missing-texture placeholder) rather than aborting the whole run, so
	// one corrupt asset never blocks the others.
	textures, err := pixel.BuildRegistry(cfg.Assets)
	if err != nil {
		log.Error("build texture registry", "error", err)
	}

	fonts, err := typeface.BuildRegistry(cfg.Assets)
	if err != nil {
		log.Error("build font registry", "error", err)
	}

	cam := cfg.Camera
	cam.Viewport = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(opts.Width), Y: float64(opts.Height)})

	r := Renderer{
		Width:    opts.Width,
		Height:   opts.Height,
		Camera:   cam,
		Clear:    cfg.Clear,
		Textures: textures,
		Fonts:    fonts,
		Assets:   cfg.Assets,
	}

	if opts.PNG != "" {
		img := renderFrame(&r, driver, textures, 0)
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
		frameDur := time.Duration(opts.FrameDT * float64(time.Second))
		for frame := range opts.Frames {
			img := renderFrame(&r, driver, textures, frameDur)
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

func renderFrame(r *Renderer, driver *runtime.Driver, textures pixel.Registry, elapsed time.Duration) *image.RGBA {
	frame := driver.Frame(elapsed)
	applyTextureUpdates(textures, driver.ConsumeTextureUpdates())
	return r.Render(frame.Commands)
}

func applyTextureUpdates(reg pixel.Registry, updates []runtime.TextureUpdate) {
	for _, u := range updates {
		if int(u.ID) < len(reg) && reg[u.ID] != nil && u.Image != nil {
			draw.Draw(reg[u.ID], reg[u.ID].Bounds(), u.Image, image.Point{}, draw.Src)
		}
	}
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
	err = fn(f)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func toPaletted(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	p := image.NewPaletted(bounds, palette.Plan9)
	draw.FloydSteinberg.Draw(p, bounds, img, image.Point{})
	return p
}

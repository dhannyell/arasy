package cli

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const defaultAtlasMaxSize = 8192

var atlasFlags struct {
	output  string
	columns int
	maxSize int
}

var atlasCmd = &cobra.Command{
	Use:   "atlas <frames-dir|input.gif>",
	Short: "pack a frame sequence into a grid sprite sheet for the manifest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAtlas(cmd, args[0], atlasFlags.output, atlasFlags.columns, atlasFlags.maxSize)
	},
}

func init() {
	atlasCmd.Flags().StringVarP(&atlasFlags.output, "output", "o", "", "output grid PNG path")
	atlasCmd.Flags().IntVar(&atlasFlags.columns, "columns", 0, "grid columns (default: ceil(sqrt(frame count)))")
	atlasCmd.Flags().IntVar(&atlasFlags.maxSize, "max-size", defaultAtlasMaxSize, "maximum atlas dimension per axis, in pixels")
	_ = atlasCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(atlasCmd)
}

func runAtlas(cmd *cobra.Command, input, output string, columns, maxSize int) error {
	frames, rate, err := loadAtlasFrames(input)
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("atlas: %q has no frames", input)
	}

	cellW, cellH := frames[0].Bounds().Dx(), frames[0].Bounds().Dy()
	if cellW <= 0 || cellH <= 0 {
		return fmt.Errorf("atlas: frame 0 has empty bounds %v", frames[0].Bounds())
	}
	for i, frame := range frames {
		if w, h := frame.Bounds().Dx(), frame.Bounds().Dy(); w != cellW || h != cellH {
			return fmt.Errorf("atlas: frame %d is %dx%d, want %dx%d (every frame must share one size)", i, w, h, cellW, cellH)
		}
	}

	if columns <= 0 {
		columns = int(math.Ceil(math.Sqrt(float64(len(frames)))))
	}
	rows := len(frames) / columns
	if len(frames)%columns != 0 {
		rows++
	}

	if maxSize <= 0 {
		maxSize = defaultAtlasMaxSize
	}
	width, err := checkedAtlasDimension("width", "columns", "cell_w", columns, cellW, maxSize)
	if err != nil {
		return err
	}
	height, err := checkedAtlasDimension("height", "rows", "cell_h", rows, cellH, maxSize)
	if err != nil {
		return err
	}
	if width > math.MaxInt/4 || height > math.MaxInt/(width*4) {
		return fmt.Errorf("atlas: grid %dx%d exceeds the addressable NRGBA image size", width, height)
	}

	sheet := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i, frame := range frames {
		col, row := i%columns, i/columns
		dest := image.Rect(col*cellW, row*cellH, (col+1)*cellW, (row+1)*cellH)
		draw.Draw(sheet, dest, frame, frame.Bounds().Min, draw.Src)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("atlas: create output directory: %w", err)
	}
	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("atlas: create %q: %w", output, err)
	}
	if err := png.Encode(out, sheet); err != nil {
		_ = out.Close()
		return fmt.Errorf("atlas: encode %q: %w", output, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("atlas: close %q: %w", output, err)
	}

	stdout := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(stdout, "cell_w: %d\ncell_h: %d\ncolumns: %d\nframes: %d\n", cellW, cellH, columns, len(frames)); err != nil {
		return err
	}
	if rate > 0 {
		_, err = fmt.Fprintf(stdout, "rate: %g\n", rate)
	}
	return err
}

func checkedAtlasDimension(axis, cellsName, cellSizeName string, cells, cellSize, maxSize int) (int, error) {
	if cells > maxSize/cellSize {
		hint := "use more columns or smaller frames"
		if axis == "width" {
			hint = "use fewer columns or smaller frames"
		}
		return 0, fmt.Errorf(
			"atlas: grid %s (%s=%d * %s=%d) exceeds --max-size %d; %s",
			axis, cellsName, cells, cellSizeName, cellSize, maxSize, hint,
		)
	}
	return cells * cellSize, nil
}

// loadAtlasFrames reads a frame sequence from a directory of PNGs (lexical
// order) or a single animated GIF, returning the decoded frames and, for a
// GIF source, a suggested playback rate in frames per second derived from
// the average frame delay (0 for directory input, which has no timing).
func loadAtlasFrames(input string) ([]image.Image, float64, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, 0, fmt.Errorf("atlas: %w", err)
	}

	if info.IsDir() {
		frames, err := loadAtlasFrameDir(input)
		return frames, 0, err
	}

	if strings.EqualFold(filepath.Ext(input), ".gif") {
		return loadAtlasFrameGIF(input)
	}

	return nil, 0, fmt.Errorf("atlas: %q is neither a directory nor a .gif file", input)
}

func loadAtlasFrameDir(dir string) ([]image.Image, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("atlas: read %q: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".png" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	frames := make([]image.Image, 0, len(names))
	for _, name := range names {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("atlas: open %q: %w", name, err)
		}
		img, err := png.Decode(f)
		closeErr := f.Close()
		if err != nil {
			return nil, fmt.Errorf("atlas: decode %q: %w", name, err)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		frames = append(frames, img)
	}
	return frames, nil
}

func loadAtlasFrameGIF(path string) ([]image.Image, float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("atlas: open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, 0, fmt.Errorf("atlas: decode %q: %w", path, err)
	}
	if len(g.Image) == 0 {
		return nil, 0, fmt.Errorf("atlas: GIF %q has no frames", path)
	}

	frames := make([]image.Image, 0, len(g.Image))
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	if bounds.Empty() {
		return nil, 0, fmt.Errorf("atlas: GIF %q has an empty logical canvas %v", path, bounds)
	}
	canvas := image.NewNRGBA(bounds)
	background := gifBackgroundColor(g)
	draw.Draw(canvas, bounds, image.NewUniform(background), image.Point{}, draw.Src)
	var totalDelay int
	for i, frame := range g.Image {
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		var previous *image.NRGBA
		if disposal == gif.DisposalPrevious {
			previous = cloneNRGBA(canvas)
		}

		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		frames = append(frames, cloneNRGBA(canvas))

		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = previous
		}
	}
	for _, delay := range g.Delay {
		totalDelay += delay
	}

	var rate float64
	if len(g.Delay) > 0 && totalDelay > 0 {
		avgDelay := float64(totalDelay) / float64(len(g.Delay)) // 1/100s units
		rate = 100 / avgDelay
	}
	return frames, rate, nil
}

func gifBackgroundColor(g *gif.GIF) color.Color {
	palette, ok := g.Config.ColorModel.(color.Palette)
	if !ok || int(g.BackgroundIndex) >= len(palette) {
		return color.Transparent
	}
	return palette[g.BackgroundIndex]
}

func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	clone := image.NewNRGBA(src.Bounds())
	draw.Draw(clone, clone.Bounds(), src, src.Bounds().Min, draw.Src)
	return clone
}

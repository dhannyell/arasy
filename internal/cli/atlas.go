package cli

import (
	"fmt"
	"image"
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
	for i, frame := range frames {
		if w, h := frame.Bounds().Dx(), frame.Bounds().Dy(); w != cellW || h != cellH {
			return fmt.Errorf("atlas: frame %d is %dx%d, want %dx%d (every frame must share one size)", i, w, h, cellW, cellH)
		}
	}

	if columns <= 0 {
		columns = int(math.Ceil(math.Sqrt(float64(len(frames)))))
	}
	rows := int(math.Ceil(float64(len(frames)) / float64(columns)))

	if maxSize <= 0 {
		maxSize = defaultAtlasMaxSize
	}
	if w := columns * cellW; w > maxSize {
		return fmt.Errorf("atlas: grid width %d (columns=%d * cell_w=%d) exceeds --max-size %d; use more columns or smaller frames", w, columns, cellW, maxSize)
	}
	if h := rows * cellH; h > maxSize {
		return fmt.Errorf("atlas: grid height %d (rows=%d * cell_h=%d) exceeds --max-size %d; use more columns or smaller frames", h, rows, cellH, maxSize)
	}

	sheet := image.NewNRGBA(image.Rect(0, 0, columns*cellW, rows*cellH))
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

	frames := make([]image.Image, 0, len(g.Image))
	bounds := g.Image[0].Bounds()
	canvas := image.NewNRGBA(bounds)
	var totalDelay int
	for _, frame := range g.Image {
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		snapshot := image.NewNRGBA(bounds)
		draw.Draw(snapshot, bounds, canvas, bounds.Min, draw.Src)
		frames = append(frames, snapshot)
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

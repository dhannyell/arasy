package cli

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func writeAtlasFramePNG(t *testing.T, dir, name string, c color.Color) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestAtlasPacksEqualFramesIntoGrid(t *testing.T) {
	dir := t.TempDir()
	colors := []color.NRGBA{
		{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255},
	}
	for i, c := range colors {
		writeAtlasFramePNG(t, dir, "frame-"+string(rune('0'+i))+".png", c)
	}

	output := filepath.Join(t.TempDir(), "atlas.png")
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	if err := runAtlas(cmd, dir, output, 4, defaultAtlasMaxSize); err != nil {
		t.Fatalf("runAtlas: %v", err)
	}

	sheet := decodePNG(t, output)
	bounds := sheet.Bounds()
	if bounds.Dx() != 8 || bounds.Dy() != 2 {
		t.Fatalf("atlas size = %dx%d, want 8x2 (4 columns of 2x2 cells)", bounds.Dx(), bounds.Dy())
	}
	for i, want := range colors {
		got := sheet.At(i*2, 0)
		wr, wg, wb, wa := want.RGBA()
		gr, gg, gb, ga := got.RGBA()
		if wr != gr || wg != gg || wb != gb || wa != ga {
			t.Fatalf("cell %d color = %+v, want %+v", i, got, want)
		}
	}

	out := stdout.String()
	for _, want := range []string{"cell_w: 2\n", "cell_h: 2\n", "columns: 4\n", "frames: 4\n"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("stdout = %q, missing %q", out, want)
		}
	}
}

func TestAtlasRejectsMismatchedFrameSizes(t *testing.T) {
	dir := t.TempDir()
	writeAtlasFramePNG(t, dir, "frame-0.png", color.NRGBA{255, 0, 0, 255})

	big := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	f, err := os.Create(filepath.Join(dir, "frame-1.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, big); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	err = runAtlas(cmd, dir, filepath.Join(t.TempDir(), "atlas.png"), 0, defaultAtlasMaxSize)
	if err == nil {
		t.Fatal("expected an error for mismatched frame sizes, got nil")
	}
}

func TestAtlasRejectsMaxSizeOverflow(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 4; i++ {
		writeAtlasFramePNG(t, dir, "frame-"+string(rune('0'+i))+".png", color.NRGBA{255, 0, 0, 255})
	}

	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	err := runAtlas(cmd, dir, filepath.Join(t.TempDir(), "atlas.png"), 1, 3)
	if err == nil {
		t.Fatal("expected an error when the grid exceeds --max-size, got nil")
	}
}

func TestAtlasRejectsGridDimensionIntegerOverflow(t *testing.T) {
	dir := t.TempDir()
	writeAtlasFramePNG(t, dir, "frame.png", color.NRGBA{255, 0, 0, 255})

	tests := []struct {
		name    string
		columns int
		maxSize int
	}{
		{
			name:    "dimension multiplication",
			columns: math.MaxInt,
			maxSize: defaultAtlasMaxSize,
		},
		{
			name:    "pixel buffer multiplication",
			columns: math.MaxInt / 2,
			maxSize: math.MaxInt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetOut(new(bytes.Buffer))
			err := runAtlas(
				cmd,
				dir,
				filepath.Join(t.TempDir(), "atlas.png"),
				tt.columns,
				tt.maxSize,
			)
			if err == nil {
				t.Fatal("expected an error when grid dimensions overflow, got nil")
			}
		})
	}
}

func TestAtlasDefaultColumnsIsCeilSqrt(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 4; i++ {
		writeAtlasFramePNG(t, dir, "frame-"+string(rune('0'+i))+".png", color.NRGBA{255, 0, 0, 255})
	}

	output := filepath.Join(t.TempDir(), "atlas.png")
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	if err := runAtlas(cmd, dir, output, 0, defaultAtlasMaxSize); err != nil {
		t.Fatalf("runAtlas: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("columns: 2\n")) {
		t.Fatalf("stdout = %q, want columns: 2 (ceil(sqrt(4)))", stdout.String())
	}
}

func TestLoadAtlasFrameGIFCompositesLogicalCanvasAndDisposal(t *testing.T) {
	palette := color.Palette{
		color.NRGBA{0, 0, 0, 255},
		color.NRGBA{255, 0, 0, 255},
		color.NRGBA{0, 255, 0, 255},
		color.NRGBA{0, 0, 255, 255},
		color.NRGBA{255, 255, 0, 255},
		color.NRGBA{0, 0, 0, 0},
	}
	frames := []*image.Paletted{
		palettedFrame(image.Rect(0, 0, 1, 1), palette, []uint8{1}),
		palettedFrame(image.Rect(1, 0, 2, 1), palette, []uint8{2}),
		palettedFrame(image.Rect(2, 0, 3, 1), palette, []uint8{3}),
		palettedFrame(image.Rect(0, 0, 1, 1), palette, []uint8{4}),
		palettedFrame(image.Rect(0, 0, 2, 1), palette, []uint8{5, 2}),
	}
	input := filepath.Join(t.TempDir(), "partial.gif")
	f, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	err = gif.EncodeAll(f, &gif.GIF{
		Image:           frames,
		Delay:           []int{10, 10, 10, 10, 10},
		Disposal:        []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious, gif.DisposalNone, gif.DisposalNone},
		Config:          image.Config{ColorModel: palette, Width: 3, Height: 1},
		BackgroundIndex: 0,
	})
	closeErr := f.Close()
	if err != nil {
		t.Fatalf("encode GIF: %v", err)
	}
	if closeErr != nil {
		t.Fatalf("close GIF: %v", closeErr)
	}

	gotFrames, gotRate, err := loadAtlasFrameGIF(input)
	if err != nil {
		t.Fatalf("loadAtlasFrameGIF: %v", err)
	}
	if gotRate != 10 {
		t.Fatalf("rate = %g, want 10", gotRate)
	}

	black := color.NRGBA{0, 0, 0, 255}
	red := color.NRGBA{255, 0, 0, 255}
	green := color.NRGBA{0, 255, 0, 255}
	blue := color.NRGBA{0, 0, 255, 255}
	yellow := color.NRGBA{255, 255, 0, 255}
	want := [][]color.NRGBA{
		{red, black, black},
		{red, green, black},
		{red, black, blue},
		{yellow, black, black},
		{yellow, green, black},
	}
	if len(gotFrames) != len(want) {
		t.Fatalf("frame count = %d, want %d", len(gotFrames), len(want))
	}
	for i, frame := range gotFrames {
		if frame.Bounds() != image.Rect(0, 0, 3, 1) {
			t.Fatalf("frame %d bounds = %v, want global 3x1 canvas", i, frame.Bounds())
		}
		for x, wantColor := range want[i] {
			gotColor := color.NRGBAModel.Convert(frame.At(x, 0)).(color.NRGBA)
			if gotColor != wantColor {
				t.Fatalf("frame %d pixel %d = %#v, want %#v", i, x, gotColor, wantColor)
			}
		}
	}
}

func palettedFrame(bounds image.Rectangle, palette color.Palette, pixels []uint8) *image.Paletted {
	frame := image.NewPaletted(bounds, palette)
	copy(frame.Pix, pixels)
	return frame
}

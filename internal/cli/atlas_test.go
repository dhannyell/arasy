package cli

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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

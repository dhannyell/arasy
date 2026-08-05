package cli

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// The stencil's promise is dimensional: one ground tile comes out exactly
// --tile pixels wide, because an artist calibrates their canvas against it.
// The only honest check is measuring the rendered sheet, so this finds the
// unit cube's top face -- the one shape drawn in stencilCubeTop, a 1x1 ground
// tile lifted to z=1 -- and reads its bounding box. Under the default 2:1
// view a tile is a diamond --tile wide and half that tall.
func TestStencilSheetDrawsTheTileAtTheAskedScale(t *testing.T) {
	out := filepath.Join(t.TempDir(), "stencil.png")
	rootCmd.SetArgs([]string{"stencil", "-o", out})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sheet, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode sheet: %v", err)
	}

	width, height, found := colorBounds(sheet, 170, 176, 192)
	if !found {
		t.Fatal("the cube's top face is missing from the sheet")
	}

	const tile = 64 // the --tile default
	if math.Abs(float64(width-tile)) > 2 {
		t.Errorf("tile is %d px wide on the sheet, want %d", width, tile)
	}
	if math.Abs(float64(height-tile/2)) > 2 {
		t.Errorf("tile is %d px tall on the sheet, want %d under the 2:1 view", height, tile/2)
	}
}

// colorBounds returns the pixel extent of the given opaque color, used to
// measure a shape on the finished sheet rather than trust the numbers that
// sized it.
func colorBounds(img image.Image, wantR, wantG, wantB uint32) (width, height int, found bool) {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a != 0xffff || r>>8 != wantR || g>>8 != wantG || b>>8 != wantB {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}
	if maxX < minX {
		return 0, 0, false
	}
	return maxX - minX + 1, maxY - minY + 1, true
}

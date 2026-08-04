package cli

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/arasy/render"
)

// The stencil's promise is dimensional: the sheet is sized so one ground tile
// comes out exactly --tile pixels wide. If the framing or scale derivation
// drifts, an artist calibrates their canvas against a lie.
func TestStencilSheetHonoursTheTileScale(t *testing.T) {
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
	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode sheet: %v", err)
	}

	wantW, wantH := stencilSheetSize(mustStencilCamera(t).View)
	if config.Width != wantW || config.Height != wantH {
		t.Fatalf("sheet is %dx%d, want %dx%d (content at the asked tile scale)",
			config.Width, config.Height, wantW, wantH)
	}
}

func mustStencilCamera(t *testing.T) render.Camera {
	t.Helper()
	camera, err := stencilCamera()
	if err != nil {
		t.Fatal(err)
	}
	return camera
}

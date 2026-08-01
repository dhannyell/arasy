package tabs

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

const goldenChannelTolerance = uint8(1)

// TestGoldenRaster is the visual oracle for the widget: one committed frame
// per tab, rendered headlessly so it runs on CI without a browser or a GPU.
// It is what catches a regression the command-level unit tests cannot see —
// a header drawn at the wrong x, a body that keeps the previous tab's
// content, a theme slot silently left zero.
func TestGoldenRaster(t *testing.T) {
	for tab, name := range map[int]string{
		tabScene:  "tabs-scene",
		tabAssets: "tabs-assets",
		tabLog:    "tabs-log",
	} {
		t.Run(name, func(t *testing.T) {
			game := &Game{}
			driver := runtime.NewDriver(game, slog.New(slog.NewTextHandler(io.Discard, nil)))
			driver.Step()
			clickAt(driver, headerCenter(tab))
			if game.State.ActiveTab != tab {
				t.Fatalf("active tab = %d, want %d", game.State.ActiveTab, tab)
			}

			assertGolden(t, name, goldenRenderer().Render(driver.Frame(0).Commands))
		})
	}
}

func goldenRenderer() *raster.Renderer {
	view := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: viewW, Y: viewH})
	return &raster.Renderer{
		Width:  viewW,
		Height: viewH,
		Camera: render.Camera{World: view, Viewport: view, Aspect: render.AspectFit},
		Clear:  (&Game{}).Setup().Clear,
	}
}

func assertGolden(t *testing.T, name string, got *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", name+".png")

	if *update {
		writePNG(t, path, got)
		t.Logf("updated %s", path)
		return
	}

	want := readPNG(t, path)
	if want.Bounds() != got.Bounds() {
		t.Fatalf("%s: bounds %v, want %v", name, got.Bounds(), want.Bounds())
	}
	for i := range want.Pix {
		if channelWithinTolerance(want.Pix[i], got.Pix[i], goldenChannelTolerance) {
			continue
		}
		failure := filepath.Join("testdata", "failures", name+".png")
		writePNG(t, failure, got)
		t.Fatalf("%s: rendering differs from the golden; got image saved to %s "+
			"(if the change is intentional, regenerate with -update and explain the diff in the PR)",
			name, failure)
	}
}

func channelWithinTolerance(want, got, tolerance uint8) bool {
	if want >= got {
		return want-got <= tolerance
	}
	return got-want <= tolerance
}

func readPNG(t *testing.T, path string) *image.RGBA {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./examples/tabs -run TestGoldenRaster -update)", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}
	rgba, ok := img.(*image.RGBA)
	if ok {
		return rgba
	}
	out := image.NewRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}

func writePNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

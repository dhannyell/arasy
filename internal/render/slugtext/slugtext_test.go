package slugtext_test

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	slug "github.com/dhannyell/go-slug"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/internal/render/slugtext"
	"github.com/dhannyell/arasy/render"
)

// A full Pack must keep the frame's entries valid, and recover in the next frame.
func TestFullPackResetsAtEndOfFrame(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	e := slugtext.New(func(render.FontID) *typeface.Face { return face })
	cmd := render.DrawText{Text: "abc", Size: 16, Font: 1, Color: render.RGBA(1, 1, 1, 1)}
	cam := render.Camera{}
	glyphs, err := e.Append(nil, cmd, cam)
	if err != nil || len(glyphs) != 3 {
		t.Fatalf("Append = %d glyphs, %v", len(glyphs), err)
	}
	generation := e.Pack().Generation()

	e.Pack().MaxBytes = e.Pack().Size()
	cmd.Text = "xyz"
	if _, err = e.Append(nil, cmd, cam); !errors.Is(err, slug.ErrPackFull) {
		t.Fatalf("Append over the limit = %v", err)
	}
	if e.Pack().Generation() != generation {
		t.Fatal("the Pack reset in the frame that still uses its entries")
	}

	e.EndFrame()
	if e.Pack().Generation() == generation {
		t.Fatal("EndFrame did not reset the full Pack")
	}
	if _, err = e.Append(nil, cmd, cam); err != nil {
		t.Fatalf("Append after the reset = %v", err)
	}
}

func TestUnknownFontReportsNoFont(t *testing.T) {
	e := slugtext.New(nil)
	cmd := render.DrawText{Text: "abc", Size: 16, Font: 7, Color: render.RGBA(1, 1, 1, 1)}
	if _, err := e.Append(nil, cmd, render.Camera{}); !errors.Is(err, slugcore.ErrNoFont) {
		t.Fatalf("Append = %v", err)
	}
}

// GlyphOutlines exists to keep the text shaper out of a game that does not set it.
func TestHostsDoNotLinkTextShaper(t *testing.T) {
	cases := []struct{ goos, goarch, pkg string }{
		{runtime.GOOS, runtime.GOARCH, "github.com/dhannyell/arasy/platform/native"},
		{runtime.GOOS, runtime.GOARCH, "github.com/dhannyell/arasy/platform/raster"},
		{"js", "wasm", "github.com/dhannyell/arasy/platform/wasm"},
	}
	for _, c := range cases {
		cmd := exec.Command("go", "list", "-deps", c.pkg)
		cmd.Env = append(os.Environ(), "GOOS="+c.goos, "GOARCH="+c.goarch)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list %s: %v\n%s", c.pkg, err, stderr.String())
		}
		if strings.Contains(string(out), "go-slug/text") {
			t.Errorf("%s links go-slug/text", c.pkg)
		}
	}
}

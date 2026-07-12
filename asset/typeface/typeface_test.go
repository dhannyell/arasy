package typeface_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/render"
	"golang.org/x/image/font/gofont/goregular"
)

func TestParseAcceptsTrueTypeAndRejectsInvalidData(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		face, err := typeface.Parse(goregular.TTF)
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if face == nil || face.Font() == nil {
			t.Fatal("Parse() returned a nil font")
		}
	})

	for name, data := range map[string][]byte{
		"empty":     nil,
		"corrupt":   []byte("not a font"),
		"truncated": goregular.TTF[:64],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := typeface.Parse(data); err == nil {
				t.Fatal("Parse() error = nil, want invalid-font error")
			}
		})
	}
}

func TestMeasureIsDeterministicAndScalesLinearly(t *testing.T) {
	left := mustParse(t, goregular.TTF)
	right := mustParse(t, goregular.TTF)
	const text = "Título Ágil"

	left16 := left.Measure(text, 16)
	right16 := right.Measure(text, 16)
	if left16 != right16 {
		t.Fatalf("Measure() differs across equivalent faces: left=%v right=%v", left16, right16)
	}
	left32 := left.Measure(text, 32)
	if delta := left32.X - 2*left16.X; delta < -0.25 || delta > 0.25 {
		t.Fatalf("Measure().X did not scale linearly: size16=%v size32=%v", left16, left32)
	}
	if delta := left32.Y - 2*left16.Y; delta < -0.02 || delta > 0.02 {
		t.Fatalf("Measure() did not scale linearly: size16=%v size32=%v", left16, left32)
	}
	if got, shorter := left.Measure(text+"!", 16).X, left16.X; got <= shorter {
		t.Fatalf("Measure(%q).X = %v, want > %v", text+"!", got, shorter)
	}
}

func TestMeasureUsesProportionalAdvancesAndNotDefFallback(t *testing.T) {
	face := mustParse(t, goregular.TTF)

	if narrow, wide := face.Measure("iiii", 16).X, face.Measure("MMMM", 16).X; narrow >= wide {
		t.Fatalf("proportional widths: iiii=%v, MMMM=%v", narrow, wide)
	}
	if got := face.Measure("\U0010ffff", 16).X; got <= 0 {
		t.Fatalf("unknown-rune NotDef advance = %v, want > 0", got)
	}
}

func TestMeasureAndAscentHandleLineMetricsAndNonPositiveSizes(t *testing.T) {
	face := mustParse(t, goregular.TTF)

	measurement := face.Measure("FrameForge", 16)
	ascent := face.Ascent(16)
	if ascent <= 0 || ascent >= measurement.Y {
		t.Fatalf("Ascent(16) = %v, want within (0, line height %v)", ascent, measurement.Y)
	}
	if empty := face.Measure("", 16); empty.X != 0 || empty.Y != measurement.Y {
		t.Fatalf("Measure(empty) = %v, want zero width and line height %v", empty, measurement.Y)
	}
	for _, size := range []float64{0, -1, -100} {
		if got := face.Measure("text", size); got.X != 0 || got.Y != 0 {
			t.Errorf("Measure(size %v) = %v, want zero", size, got)
		}
		if got := face.Ascent(size); got != 0 {
			t.Errorf("Ascent(size %v) = %v, want zero", size, got)
		}
	}
}

func TestBuildRegistrySupportsLookupAndReportsPartialFailures(t *testing.T) {
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"fonts":[{"name":"broken","file":"fonts/broken.ttf"},{"name":"regular","file":"fonts/regular.ttf"}]}`)},
		"fonts/broken.ttf":  {Data: []byte("not a font")},
		"fonts/regular.ttf": {Data: goregular.TTF},
	})
	if err != nil {
		t.Fatalf("asset.Load() error = %v", err)
	}

	registry, err := typeface.BuildRegistry(manifest)
	if err == nil || !strings.Contains(err.Error(), `decode font "broken"`) {
		t.Fatalf("BuildRegistry() error = %v, want named broken-font error", err)
	}
	if got := registry.Lookup(manifest.Font("regular")); got == nil {
		t.Fatal("Lookup(regular) = nil, want decoded face despite sibling failure")
	}
	if got := registry.Lookup(manifest.Font("broken")); got != nil {
		t.Fatalf("Lookup(broken) = %v, want nil", got)
	}
	for _, id := range []render.FontID{0, 99} {
		if got := registry.Lookup(id); got != nil {
			t.Fatalf("Lookup(%d) = %v, want nil", id, got)
		}
	}
}

func TestBuildRegistryValidManifestReturnsNoError(t *testing.T) {
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"regular","file":"regular.ttf"}]}`)},
		"regular.ttf":   {Data: goregular.TTF},
	})
	if err != nil {
		t.Fatalf("asset.Load() error = %v", err)
	}

	registry, err := typeface.BuildRegistry(manifest)
	if err != nil {
		t.Fatalf("BuildRegistry() error = %v", err)
	}
	if got := registry.Lookup(1); got == nil {
		t.Fatal("Lookup(1) = nil, want decoded face")
	}
}

func mustParse(t *testing.T, data []byte) *typeface.Face {
	t.Helper()
	face, err := typeface.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return face
}

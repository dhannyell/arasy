package pixel_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/asset/pixel"
)

func TestDecodeRGBAPremultipliesAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	decoded, err := pixel.DecodeRGBA(buf.Bytes())
	if err != nil {
		t.Fatalf("DecodeRGBA() error = %v", err)
	}
	got := decoded.RGBAAt(0, 0)
	if got.R < 127 || got.R > 128 || got.A != 128 {
		t.Fatalf("premultiplied pixel = %#v, want R around 128 and A 128", got)
	}
}

func TestDecodeRGBARejectsInvalidBytes(t *testing.T) {
	if _, err := pixel.DecodeRGBA([]byte("not-an-image")); err == nil {
		t.Fatal("DecodeRGBA() error = nil, want decode failure")
	}
}

func TestBuildRegistryAndLookup(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 4})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	m, err := asset.Load(fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}]}`)},
		"textures/ball.png": {Data: buf.Bytes()},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	reg, err := pixel.BuildRegistry(m)
	if err != nil {
		t.Fatalf("BuildRegistry() error = %v", err)
	}
	if reg.Lookup(0) != nil || reg.Lookup(99) != nil {
		t.Fatalf("Lookup on invalid ids should be nil")
	}
	if got := reg.Lookup(1); got == nil {
		t.Fatal("Lookup(1) = nil, want decoded image")
	}
}

func TestBuildRegistryReportsCorruptedTexture(t *testing.T) {
	m, err := asset.Load(fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}]}`)},
		"textures/ball.png": {Data: []byte("not-a-png")},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err := pixel.BuildRegistry(m); err == nil {
		t.Fatal("BuildRegistry() error = nil, want decode failure")
	}
}

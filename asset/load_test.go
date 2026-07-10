package asset_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
)

func TestLoadSingleManifestResolvesTexturesAndSprites(t *testing.T) {
	layers := fstest.MapFS{
		"manifest.json":        {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"},{"name":"players","file":"textures/players.png"}],"sprites":[{"name":"player_idle","texture":"players","x":1,"y":2,"w":3,"h":4}],"audio":[{"id":"hit","file":"audio/hit.wav"}]}`)},
		"audio/hit.wav":        {Data: []byte("hit-bytes")},
		"textures/ball.png":    {Data: []byte("ball-bytes")},
		"textures/players.png": {Data: []byte("players-bytes")},
	}

	m, err := asset.Load(layers)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := m.Texture("ball"); got != 1 {
		t.Fatalf("Texture(ball) = %d, want 1", got)
	}
	if got := m.Texture("players"); got != 2 {
		t.Fatalf("Texture(players) = %d, want 2", got)
	}
	if got := m.Textures(); len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("Textures() = %#v, want ordered IDs 1..2", got)
	}
	if got := string(m.Textures()[0].Data); got != "ball-bytes" {
		t.Fatalf("ball bytes = %q, want ball-bytes", got)
	}
	sprite := m.Sprite("player_idle")
	if sprite.Texture != 2 || sprite.Src.Min.X != 1 || sprite.Src.Min.Y != 2 || sprite.Src.Width() != 3 || sprite.Src.Height() != 4 {
		t.Fatalf("Sprite(player_idle) = %#v, want players sprite", sprite)
	}
	clips := m.AudioClips()
	if len(clips) != 1 || clips[0].ID != "hit" || clips[0].Path != "audio/hit.wav" {
		t.Fatalf("AudioClips() = %#v, want hit clip", clips)
	}
}

func TestLoadIsDeterministicAcrossCalls(t *testing.T) {
	layers := fstest.MapFS{
		"manifest.json":      {Data: []byte(`{"version":1,"textures":[{"name":"zeta","file":"textures/zeta.bin"},{"name":"alpha","file":"textures/alpha.bin"}]}`)},
		"textures/zeta.bin":  {Data: []byte("z")},
		"textures/alpha.bin": {Data: []byte("a")},
	}

	left, err := asset.Load(layers)
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	right, err := asset.Load(layers)
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if left.Texture("alpha") != right.Texture("alpha") || left.Texture("zeta") != right.Texture("zeta") {
		t.Fatalf("manifests not deterministic: left=%#v right=%#v", left.Textures(), right.Textures())
	}
}

func TestLoadAllowsManifestOverrideAndFileOverride(t *testing.T) {
	base := fstest.MapFS{
		"audio/theme.wav":   {Data: []byte("theme-base")},
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}],"audio":[{"id":"theme","file":"audio/theme.wav"}]}`)},
		"textures/ball.png": {Data: []byte("base")},
	}
	overrideManifest := fstest.MapFS{
		"audio/theme-hi.wav":   {Data: []byte("theme-hi")},
		"manifest.json":        {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball-hi.png"}],"audio":[{"id":"theme","file":"audio/theme-hi.wav"}]}`)},
		"textures/ball-hi.png": {Data: []byte("hi")},
	}
	overrideFile := fstest.MapFS{
		"textures/ball-hi.png": {Data: []byte("file-only-hi")},
	}

	m, err := asset.Load(base, overrideManifest, overrideFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := string(m.Textures()[0].Data); got != "file-only-hi" {
		t.Fatalf("override bytes = %q, want file-only-hi", got)
	}
	if got := m.AudioClips()[0].Path; got != "audio/theme-hi.wav" {
		t.Fatalf("override audio path = %q, want audio/theme-hi.wav", got)
	}
}

func TestLoadSpriteErrorsAndValidationFailures(t *testing.T) {
	t.Run("unknown texture", func(t *testing.T) {
		_, err := asset.Load(fstest.MapFS{
			"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}],"sprites":[{"name":"s","texture":"missing","x":0,"y":0,"w":1,"h":1}]}`)},
			"textures/ball.png": {Data: []byte("b")},
		})
		if err == nil {
			t.Fatal("Load() error = nil, want unknown texture error")
		}
	})
	t.Run("no manifest", func(t *testing.T) {
		_, err := asset.Load(fstest.MapFS{})
		if err == nil {
			t.Fatal("Load() error = nil, want missing manifest error")
		}
	})
	t.Run("bad version", func(t *testing.T) {
		_, err := asset.Load(fstest.MapFS{"manifest.json": {Data: []byte(`{"version":2}`)}})
		if err == nil {
			t.Fatal("Load() error = nil, want version error")
		}
	})
	t.Run("duplicate texture", func(t *testing.T) {
		_, err := asset.Load(fstest.MapFS{"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"a"},{"name":"ball","file":"b"}]}`)}})
		if err == nil {
			t.Fatal("Load() error = nil, want duplicate texture error")
		}
	})
	t.Run("duplicate audio", func(t *testing.T) {
		_, err := asset.Load(fstest.MapFS{"manifest.json": {Data: []byte(`{"version":1,"audio":[{"id":"hit","file":"a"},{"id":"hit","file":"b"}]}`)}})
		if err == nil {
			t.Fatal("Load() error = nil, want duplicate audio error")
		}
	})
	t.Run("missing file", func(t *testing.T) {
		_, err := asset.Load(fstest.MapFS{"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}]}`)}})
		if err == nil || !strings.Contains(err.Error(), "textures/ball.png") {
			t.Fatalf("error = %v, want missing path", err)
		}
	})
}

func TestManifestZeroValueIsSafe(t *testing.T) {
	var m asset.Manifest

	if got := m.Texture("x"); got != 0 {
		t.Fatalf("Texture(x) = %d, want 0", got)
	}
	if got := m.Sprite("x"); got != (asset.Sprite{}) {
		t.Fatalf("Sprite(x) = %#v, want zero value", got)
	}
	if got := m.Textures(); len(got) != 0 {
		t.Fatalf("Textures() len = %d, want 0", len(got))
	}
}

func TestLoadUsesLastFileLayer(t *testing.T) {
	base := fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}]}`)},
		"textures/ball.png": {Data: []byte("base")},
	}
	mod := fstest.MapFS{
		"textures/ball.png": {Data: []byte("mod")},
	}

	m, err := asset.Load(base, mod)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := string(m.Textures()[0].Data); got != "mod" {
		t.Fatalf("override bytes = %q, want mod", got)
	}
}

func TestLoadCanDecodeRealPNGBytesInMapFS(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	_, err := asset.Load(fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}]}`)},
		"textures/ball.png": {Data: buf.Bytes()},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestDynamicTextureDeclarations(t *testing.T) {
	lm := fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.png"}],"dynamic":[{"name":"heatmap","w":32,"h":24}]}`)},
		"textures/ball.png": {Data: []byte("dummy")},
	}
	m, err := asset.Load(lm)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := m.Texture("heatmap"); got == 0 {
		t.Fatal("Texture(heatmap) = 0")
	}
	d, ok := m.TextureDecl(m.Texture("heatmap"))
	if !ok || !d.Dynamic || d.W != 32 || d.H != 24 {
		t.Fatalf("TextureDecl(heatmap) = %#v, want Dynamic=true, W=32, H=24", d)
	}
	if !m.IsDynamic(m.Texture("heatmap")) {
		t.Fatal("IsDynamic(heatmap) = false, want true")
	}
	if m.IsDynamic(m.Texture("ball")) {
		t.Fatal("IsDynamic(ball) = true, want false")
	}
}

func TestDynamicTextureRejectsBadDimensions(t *testing.T) {
	_, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"dynamic":[{"name":"bad","w":0,"h":0}]}`)},
	})
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

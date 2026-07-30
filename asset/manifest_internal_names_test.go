package asset_test

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/particle"
)

func TestLoadRejectsReservedAssetNames(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{
			name:     "texture",
			manifest: `{"version":1,"textures":[{"name":"__ar_texture","file":"asset.bin"}]}`,
		},
		{
			name:     "dynamic texture",
			manifest: `{"version":1,"dynamic":[{"name":"__ar_dynamic","w":1,"h":1}]}`,
		},
		{
			name:     "sprite",
			manifest: `{"version":1,"sprites":[{"name":"__ar_sprite","texture":"texture","w":1,"h":1}]}`,
		},
		{
			name:     "font",
			manifest: `{"version":1,"fonts":[{"name":"__ar_font","file":"asset.bin"}]}`,
		},
		{
			name:     "audio",
			manifest: `{"version":1,"audio":[{"id":"__ar_audio","file":"asset.bin"}]}`,
		},
		{
			name:     "effect",
			manifest: `{"version":1,"effects":[{"name":"__ar_effect","capacity":1,"lifetime":[1,1],"size_start":[1,1]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := asset.Load(fstest.MapFS{
				"manifest.json": {Data: []byte(tt.manifest)},
			})
			if err == nil {
				t.Fatal("Load() error = nil, want reserved name error")
			}
			if !strings.Contains(err.Error(), `reserved prefix "__ar_"`) {
				t.Fatalf("Load() error = %q, want reserved prefix error", err)
			}
		})
	}
}

func TestWithTextureReturnsExistingIDWithoutAppending(t *testing.T) {
	manifest := loadManifestForInternalNameTests(t)
	originalID := manifest.Texture("base")
	originalDecl, ok := manifest.TextureDecl(originalID)
	if !ok {
		t.Fatal("TextureDecl(base) not found")
	}

	got, gotID := manifest.WithTexture("base", []byte("replacement"))

	if gotID != originalID {
		t.Fatalf("WithTexture() ID = %d, want existing ID %d", gotID, originalID)
	}
	if len(got.Textures()) != len(manifest.Textures()) {
		t.Fatalf("WithTexture() texture count = %d, want %d", len(got.Textures()), len(manifest.Textures()))
	}
	gotDecl, ok := got.TextureDecl(gotID)
	if !ok {
		t.Fatal("WithTexture() existing declaration not found")
	}
	if !bytes.Equal(gotDecl.Data, originalDecl.Data) {
		t.Fatalf("WithTexture() replaced existing data: got %q, want %q", gotDecl.Data, originalDecl.Data)
	}
}

func TestWithEffectReturnsExistingIDWithoutAppending(t *testing.T) {
	manifest := loadManifestForInternalNameTests(t)
	originalID := manifest.Effect("glow")
	originalDecl, ok := manifest.EffectDecl(originalID)
	if !ok {
		t.Fatal("EffectDecl(glow) not found")
	}

	got, gotID := manifest.WithEffect("glow", particle.EffectDecl{Capacity: 99})

	if gotID != originalID {
		t.Fatalf("WithEffect() ID = %d, want existing ID %d", gotID, originalID)
	}
	if len(got.Effects()) != len(manifest.Effects()) {
		t.Fatalf("WithEffect() effect count = %d, want %d", len(got.Effects()), len(manifest.Effects()))
	}
	gotDecl, ok := got.EffectDecl(gotID)
	if !ok {
		t.Fatal("WithEffect() existing declaration not found")
	}
	if gotDecl != originalDecl {
		t.Fatalf("WithEffect() replaced existing declaration: got %#v, want %#v", gotDecl, originalDecl)
	}
}

func loadManifestForInternalNameTests(t *testing.T) asset.Manifest {
	t.Helper()
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{
			"version": 1,
			"textures": [{"name": "base", "file": "base.bin"}],
			"effects": [{
				"name": "glow",
				"capacity": 1,
				"lifetime": [1, 1],
				"size_start": [1, 1]
			}]
		}`)},
		"base.bin": {Data: []byte("original")},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return manifest
}

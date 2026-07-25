package asset_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
)

func TestLoadFontsAssignsDeterministicIndependentIDs(t *testing.T) {
	layer := fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"alpha","file":"alpha.png"},{"name":"zeta","file":"zeta.png"}],"fonts":[{"name":"zeta","file":"zeta.ttf"},{"name":"alpha","file":"alpha.ttf"}]}`)},
		"alpha.png":     {Data: []byte("alpha texture")},
		"zeta.png":      {Data: []byte("zeta texture")},
		"alpha.ttf":     {Data: []byte("alpha font")},
		"zeta.ttf":      {Data: []byte("zeta font")},
	}

	first, err := asset.Load(layer)
	if err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	second, err := asset.Load(layer)
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}

	if got := first.Font("alpha"); got != 1 {
		t.Fatalf("Font(alpha) = %d, want 1", got)
	}
	if got := first.Font("zeta"); got != 2 {
		t.Fatalf("Font(zeta) = %d, want 2", got)
	}
	if first.Font("alpha") != second.Font("alpha") || first.Font("zeta") != second.Font("zeta") {
		t.Fatalf("font IDs differ across loads: first=%#v second=%#v", first.Fonts(), second.Fonts())
	}
	if first.Font("alpha") != 1 || first.Texture("alpha") != 1 {
		t.Fatalf("font and texture ID spaces are not independent: font=%d texture=%d", first.Font("alpha"), first.Texture("alpha"))
	}

	decls := first.Fonts()
	if len(decls) != 2 || decls[0].ID != 1 || decls[0].Name != "alpha" || string(decls[0].Data) != "alpha font" || decls[1].ID != 2 || decls[1].Name != "zeta" {
		t.Fatalf("Fonts() = %#v, want declarations ordered by name with IDs 1..2", decls)
	}
}

func TestLoadFontsUsesLastManifestEntryAndFileLayer(t *testing.T) {
	base := fstest.MapFS{
		"manifest.json":  {Data: []byte(`{"version":1,"fonts":[{"name":"heading","file":"fonts/base.ttf"}]}`)},
		"fonts/base.ttf": {Data: []byte("base")},
	}
	overrideManifest := fstest.MapFS{
		"manifest.json":      {Data: []byte(`{"version":1,"fonts":[{"name":"heading","file":"fonts/override.ttf"}]}`)},
		"fonts/override.ttf": {Data: []byte("manifest override")},
	}
	overrideFile := fstest.MapFS{
		"fonts/override.ttf": {Data: []byte("file override")},
	}

	manifest, err := asset.Load(base, overrideManifest, overrideFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := string(manifest.Fonts()[0].Data); got != "file override" {
		t.Fatalf("heading bytes = %q, want file override", got)
	}
}

func TestLoadFontsRejectsInvalidDeclarationsAndFiles(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		files    fstest.MapFS
		want     []string
	}{
		{
			name:     "duplicate name",
			manifest: `{"version":1,"fonts":[{"name":"body","file":"a.ttf"},{"name":"body","file":"b.ttf"}]}`,
			want:     []string{`duplicate font "body"`},
		},
		{
			name:     "empty name",
			manifest: `{"version":1,"fonts":[{"name":"","file":"font.ttf"}]}`,
			want:     []string{"font name must not be empty"},
		},
		{
			name:     "empty path",
			manifest: `{"version":1,"fonts":[{"name":"body","file":""}]}`,
			want:     []string{`font "body"`, "empty file path"},
		},
		{
			name:     "missing file",
			manifest: `{"version":1,"fonts":[{"name":"body","file":"fonts/missing.ttf"}]}`,
			want:     []string{`font "body"`, "fonts/missing.ttf"},
		},
		{
			name:     "empty file",
			manifest: `{"version":1,"fonts":[{"name":"body","file":"fonts/empty.ttf"}]}`,
			files:    fstest.MapFS{"fonts/empty.ttf": {Data: []byte{}}},
			want:     []string{`font "body"`, `file "fonts/empty.ttf" is empty`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layer := fstest.MapFS{"manifest.json": {Data: []byte(tt.manifest)}}
			for name, file := range tt.files {
				layer[name] = file
			}
			_, err := asset.Load(layer)
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Load() error = %q, want substring %q", err, want)
				}
			}
		})
	}
}

func TestLoadManifestWithoutFontsAndZeroManifestAreSafe(t *testing.T) {
	loaded, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1}`)},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for name, manifest := range map[string]asset.Manifest{"loaded": loaded, "zero": {}} {
		t.Run(name, func(t *testing.T) {
			if got := manifest.Font("missing"); got != 0 {
				t.Fatalf("Font(missing) = %d, want 0", got)
			}
			if got := manifest.Fonts(); len(got) != 0 {
				t.Fatalf("Fonts() = %#v, want empty", got)
			}
		})
	}
}

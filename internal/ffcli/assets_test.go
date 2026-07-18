package ffcli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssetsValidateLoadsManifest(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "examples", "breakout", "assets")
	manifest, err := validateAssetDirs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.AudioClips()) != 4 {
		t.Fatalf("audio clips = %d, want 4", len(manifest.AudioClips()))
	}
}

func TestAssetsValidateRejectsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := validateAssetDirs([]string{dir}); err == nil {
		t.Fatal("asset.Load accepted invalid manifest")
	}
}

func TestAssetsValidateLayersLastManifestWins(t *testing.T) {
	base := t.TempDir()
	override := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "manifest.json"), []byte(`{"version":1,"dynamic":[{"name":"canvas","w":1,"h":1}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(override, "manifest.json"), []byte(`{"version":1,"dynamic":[{"name":"canvas","w":2,"h":2}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := validateAssetDirs([]string{base, override})
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Textures()[0].W; got != 2 {
		t.Fatalf("winning texture width = %d, want 2", got)
	}
}

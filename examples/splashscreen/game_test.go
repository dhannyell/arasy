package splashscreen

import "testing"

func TestNewGameDeclaresOneCustomSplashSegment(t *testing.T) {
	game, err := NewGame()
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}

	cfg := game.Setup()
	if cfg.DisableEngineSplash {
		t.Fatal("DisableEngineSplash = true, want false (the example shows the engine default segment too)")
	}

	splash := cfg.Assets.Splash()
	if len(splash) != 1 {
		t.Fatalf("len(Splash()) = %d, want 1", len(splash))
	}
	decl := splash[0]
	if decl.Texture != "studio-logo" {
		t.Fatalf("Texture = %q, want %q", decl.Texture, "studio-logo")
	}
	if decl.Columns != 3 || decl.Frames != 6 {
		t.Fatalf("Columns/Frames = %d/%d, want 3/6", decl.Columns, decl.Frames)
	}
	if cfg.Assets.Texture(decl.Texture) == 0 {
		t.Fatal("studio-logo texture did not resolve to a non-zero TextureID")
	}
}

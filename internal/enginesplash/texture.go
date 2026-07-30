package enginesplash

import (
	_ "embed"

	"github.com/dhannyell/arasy/asset"
)

// EngineMoteTextureName and EngineEmberTextureName are the manifest texture
// names the boot splash's particles draw with: a soft round point of light
// (starfield, Crux) and a small hot spark (the ring spark). Neither carries
// brand identity, so both are generated procedurally and embedded here
// rather than depending on a private asset — see
// internal/enginesplash/texturegen for the generator that produced
// assets/mote.png and assets/ember.png (regenerate with
// `go run ./internal/enginesplash/texturegen/gen`).
const (
	EngineMoteTextureName  = asset.ReservedNamePrefix + "engine_mote"
	EngineEmberTextureName = asset.ReservedNamePrefix + "engine_ember"
)

//go:embed assets/mote.png
var engineMoteTexturePNG []byte

// EngineMoteTexture returns the mote texture's encoded PNG bytes.
func EngineMoteTexture() []byte { return engineMoteTexturePNG }

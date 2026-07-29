package enginesplash

import _ "embed"

// EngineMoteTextureName and EngineEmberTextureName are the manifest texture
// names the boot splash's particles draw with: a soft round point of light
// (starfield, Crux) and a small hot spark (the ring spark). Neither carries
// brand identity, so both are generated procedurally and embedded here
// rather than depending on a private asset — see
// internal/enginesplash/texturegen for the generator that produced
// assets/mote.png and assets/ember.png (regenerate with
// `go run ./internal/enginesplash/texturegen/gen`).
const (
	EngineMoteTextureName  = "__ar_engine_mote"
	EngineEmberTextureName = "__ar_engine_ember"
)

//go:embed assets/mote.png
var engineMoteTexturePNG []byte

//go:embed assets/ember.png
var engineEmberTexturePNG []byte

// EngineMoteTexture returns the mote texture's encoded PNG bytes.
func EngineMoteTexture() []byte { return engineMoteTexturePNG }

// EngineEmberTexture returns the ember texture's encoded PNG bytes.
func EngineEmberTexture() []byte { return engineEmberTexturePNG }

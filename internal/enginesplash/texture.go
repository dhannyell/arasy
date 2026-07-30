package enginesplash

import (
	_ "embed"

	"github.com/dhannyell/arasy/asset"
)

const engineMoteTextureName = asset.ReservedNamePrefix + "engine_mote"

//go:embed assets/mote.png
var engineMoteTexturePNG []byte

// MoteTexture returns the encoded mote used to bake static brand assets.
func MoteTexture() []byte { return engineMoteTexturePNG }

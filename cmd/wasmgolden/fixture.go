package wasmgolden

import (
	"fmt"

	"github.com/dhannyell/arasy/runtime"
)

// NewFixture builds a named renderer-conformance fixture.
func NewFixture(name string) (runtime.Game, error) {
	switch name {
	case "":
		return New()
	case "particles":
		return NewParticleGolden()
	case "sprite-array":
		return NewSpriteArrayGolden()
	case "retained-batches":
		return NewRetainedBatchesGolden()
	case "view":
		return NewViewConformanceGolden()
	case "parallax":
		return NewParallaxConformanceGolden()
	case "flip":
		return NewFlipConformanceGolden()
	case "passes":
		return NewCameraPassesGolden()
	case "post":
		return NewPostGolden()
	case "rtt":
		return NewRTTGolden()
	case "rtt-array":
		return NewRTTArrayGolden()
	case "boot":
		return NewBoot()
	default:
		return nil, fmt.Errorf("wasmgolden: unknown fixture %q", name)
	}
}

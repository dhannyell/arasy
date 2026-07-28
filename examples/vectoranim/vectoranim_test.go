package vectoranim

import (
	"crypto/sha256"
	"testing"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

func TestGeneratedAnimationRendersAtFixedTimes(t *testing.T) {
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180})
	renderer := raster.Renderer{
		Width: 320, Height: 180,
		Camera: render.Camera{World: viewport, Viewport: viewport, Aspect: render.AspectFit},
		Clear:  render.RGBA8(16, 24, 39, 255),
	}
	times := []float64{0, 0.18, 0.36, 0.75, 1.05, 1.5, 2.16, 2.6, 3}
	seen := make(map[[32]byte]bool)
	for _, elapsed := range times {
		timeline := anim.At(elapsed)
		commands := AppendVectorScene(nil, timeline, viewport, VectorSceneResources{})
		commands = AppendManualAccent(commands, timeline)
		if len(commands) == 0 {
			t.Fatalf("elapsed %g produced no commands", elapsed)
		}
		hash := sha256.Sum256(renderer.Render(commands).Pix)
		seen[hash] = true
	}
	if len(seen) < 5 {
		t.Fatalf("fixed-time raster samples produced only %d distinct frames", len(seen))
	}
}

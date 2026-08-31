//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestPlanNativeSpriteBatchGroupsMissingTexturesOnFallback(t *testing.T) {
	dest := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})
	plan := planNativeSpriteBatch([]render.SpriteInstance{
		{Texture: 7, Dest: dest},
		{Texture: 8, Dest: dest},
		{Texture: 1, Dest: dest},
	}, tess.TexSizes{{}, {16, 16}})

	if len(plan.ranges) != 2 {
		t.Fatalf("ranges = %+v, want one fallback range followed by one texture range", plan.ranges)
	}
	if got := plan.ranges[0]; !got.missing || got.texture != 0 || got.start != 0 || got.count != 2 {
		t.Fatalf("fallback range = %+v, want two missing sprites", got)
	}
	if got := plan.ranges[1]; got.missing || got.texture != 1 || got.start != 2 || got.count != 1 {
		t.Fatalf("texture range = %+v, want declared texture 1", got)
	}
}

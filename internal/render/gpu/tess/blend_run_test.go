package tess

import (
	"testing"

	"github.com/dhannyell/arasy/render"
)

// Run merging is the mechanism the GPU backends rely on for blend
// correctness: two adjacent solid commands normally share a draw, so an
// additive command must break the run or it silently draws with the wrong
// pipeline.
func TestBuildSplitsRunsOnBlendChange(t *testing.T) {
	additive := drawRect(0)
	additive.BlendMode = render.BlendAdditive

	var builder Builder
	runs := builder.Build(Frame{
		Camera:   testCamera(),
		Commands: []render.Command{drawRect(0), additive, drawRect(0)},
	}, 100, 100, nil, TexSizes{})

	wantBlends := []render.Blend{render.BlendAlpha, render.BlendAdditive, render.BlendAlpha}
	if len(runs) != len(wantBlends) {
		t.Fatalf("len(runs) = %d, want %d (adjacent solids must split on blend)", len(runs), len(wantBlends))
	}
	for i, run := range runs {
		if run.Kind != DrawRunSolid || run.Blend != wantBlends[i] {
			t.Fatalf("runs[%d] = {%v %v}, want {solid %v}", i, run.Kind, run.Blend, wantBlends[i])
		}
	}
}

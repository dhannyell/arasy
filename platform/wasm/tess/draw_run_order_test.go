package tess

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// These tests cover the heterogeneous DrawRun interleaving introduced when
// Build stopped grouping commands into three fixed passes (solid, then
// textured, then text) and started preserving painter order across kinds.
// render.Each sorts by layer before dispatch; buildSink opens a new run
// whenever the pipeline kind changes, or the texture changes within a
// textured run.

func drawRect(layer int) render.DrawRect {
	return render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Rect:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		Fill:        render.RGBA(1, 0, 0, 1),
	}
}

func drawImage(layer int, texture render.TextureID) render.DrawImage {
	return render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Texture:     texture,
		Dest:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
	}
}

func TestBuildInterleavesKindsInPainterOrderWithinSameLayer(t *testing.T) {
	sizes := TexSizes{{}, {8, 8}}

	runs := Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(0),
			drawImage(0, 1),
			drawRect(0),
		},
	}, 100, 100, nil, sizes)

	wantKinds := []DrawRunKind{DrawRunSolid, DrawRunTextured, DrawRunSolid}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d", len(runs), len(wantKinds))
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
		switch run.Kind {
		case DrawRunSolid:
			if len(run.Solid.Vertices) == 0 {
				t.Fatalf("runs[%d] solid run has no vertices", i)
			}
		case DrawRunTextured:
			if len(run.Textured.Vertices) == 0 {
				t.Fatalf("runs[%d] textured run has no vertices", i)
			}
		}
	}
}

func TestBuildSortsAcrossKindsByLayerNotInputOrder(t *testing.T) {
	sizes := TexSizes{{}, {8, 8}}

	// Input order is [rect(layer 1), image(layer 0)], the reverse of the
	// expected draw order: the lower-layer image must be dispatched first.
	runs := Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(1),
			drawImage(0, 1),
		},
	}, 100, 100, nil, sizes)

	wantKinds := []DrawRunKind{DrawRunTextured, DrawRunSolid}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d", len(runs), len(wantKinds))
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
	}
}

func TestBuildPlacesMissingTexturePlaceholderBetweenTexturedRunsWithoutMerging(t *testing.T) {
	// Texture 1 is known (8x8); texture 2 is out of range in sizes, so it
	// resolves to the magenta placeholder appended to a solid run at its
	// painter position.
	sizes := TexSizes{{}, {8, 8}}

	runs := Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawImage(0, 1),
			drawImage(0, 2),
			drawImage(0, 1),
		},
	}, 100, 100, nil, sizes)

	wantKinds := []DrawRunKind{DrawRunTextured, DrawRunSolid, DrawRunTextured}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d runs %v, got %+v", len(runs), len(wantKinds), wantKinds, runs)
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
	}

	if runs[0].Textured.Texture != 1 || runs[2].Textured.Texture != 1 {
		t.Fatalf("runs[0].Textured.Texture=%d runs[2].Textured.Texture=%d, want both 1 (not merged across the placeholder)",
			runs[0].Textured.Texture, runs[2].Textured.Texture)
	}
	if len(runs[1].Solid.Vertices) == 0 {
		t.Fatal("placeholder solid run has no vertices")
	}
}

func TestBuildInterleavesTextWithSolidsInPainterOrder(t *testing.T) {
	atlas := NewAtlas()

	runs := Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(0),
			render.DrawText{
				BaseCommand: render.BaseCommand{LayerValue: 1},
				Position:    math2d.Vec2{X: 50, Y: 50},
				Text:        "Hi",
				Color:       render.RGBA(1, 1, 1, 1),
				Size:        10,
			},
			drawRect(2),
		},
	}, 100, 100, atlas, nil)

	wantKinds := []DrawRunKind{DrawRunSolid, DrawRunText, DrawRunSolid}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d runs %v, got %+v", len(runs), len(wantKinds), wantKinds, runs)
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
	}
	if len(runs[1].Text.Vertices) == 0 {
		t.Fatal("text run has no vertices")
	}
}

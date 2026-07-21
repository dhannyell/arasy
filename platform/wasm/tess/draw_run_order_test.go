package tess

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// These tests cover Builder's heterogeneous StreamRun interleaving and
// painter order across pipeline kinds. render.Each sorts by layer before
// dispatch; streamSink opens a new run
// whenever the pipeline kind changes, or the texture changes within a sprite
// run.

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

func drawParticles(layer int, emitter render.EmitterID) render.DrawParticles {
	return render.DrawParticles{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Effect:      1,
		Emitter:     emitter,
		Dt:          1.0 / 60.0,
		Intensity:   1,
	}
}

func TestBuildInterleavesKindsInPainterOrderWithinSameLayer(t *testing.T) {
	sizes := TexSizes{{}, {8, 8}}

	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(0),
			drawImage(0, 1),
			drawRect(0),
		},
	}, 100, 100, nil, sizes)

	wantKinds := []DrawRunKind{DrawRunSolid, DrawRunSprites, DrawRunSolid}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d", len(runs), len(wantKinds))
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
		switch run.Kind {
		case DrawRunSolid:
			if run.VertexCount == 0 {
				t.Fatalf("runs[%d] solid run has no vertices", i)
			}
		case DrawRunSprites:
			if run.VertexCount == 0 {
				t.Fatalf("runs[%d] sprite run has no records", i)
			}
		}
	}
}

func TestBuildSortsAcrossKindsByLayerNotInputOrder(t *testing.T) {
	sizes := TexSizes{{}, {8, 8}}

	// Input order is [rect(layer 1), image(layer 0)], the reverse of the
	// expected draw order: the lower-layer image must be dispatched first.
	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(1),
			drawImage(0, 1),
		},
	}, 100, 100, nil, sizes)

	wantKinds := []DrawRunKind{DrawRunSprites, DrawRunSolid}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d", len(runs), len(wantKinds))
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
	}
}

func TestBuildPlacesMissingTexturePlaceholderBetweenSpriteRunsWithoutMerging(t *testing.T) {
	// Texture 1 is known (8x8); texture 2 is out of range in sizes, so it
	// resolves to the magenta placeholder appended to a solid run at its
	// painter position.
	sizes := TexSizes{{}, {8, 8}}

	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawImage(0, 1),
			drawImage(0, 2),
			drawImage(0, 1),
		},
	}, 100, 100, nil, sizes)

	wantKinds := []DrawRunKind{DrawRunSprites, DrawRunSolid, DrawRunSprites}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d runs %v, got %+v", len(runs), len(wantKinds), wantKinds, runs)
	}
	for i, run := range runs {
		if run.Kind != wantKinds[i] {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, run.Kind, wantKinds[i])
		}
	}

	if runs[0].Texture != 1 || runs[2].Texture != 1 {
		t.Fatalf("runs[0].Texture=%d runs[2].Texture=%d, want both 1 (not merged across the placeholder)",
			runs[0].Texture, runs[2].Texture)
	}
	if runs[1].VertexCount == 0 {
		t.Fatal("placeholder solid run has no vertices")
	}
}

func TestBuildInterleavesTextWithSolidsInPainterOrder(t *testing.T) {
	atlas := NewAtlas()

	var builder Builder
	runs := builder.Build(runtime.Frame{
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
	if runs[1].VertexCount == 0 {
		t.Fatal("text run has no vertices")
	}
}

func TestBuildKeepsEveryParticleCommandInItsOwnPainterOrderedRun(t *testing.T) {
	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			drawRect(0),
			drawParticles(0, 10),
			drawParticles(0, 20),
			drawRect(0),
		},
	}, 100, 100, nil, nil)

	wantKinds := []DrawRunKind{
		DrawRunSolid,
		DrawRunParticles,
		DrawRunParticles,
		DrawRunSolid,
	}
	if len(runs) != len(wantKinds) {
		t.Fatalf("len(runs) = %d, want %d; runs=%+v", len(runs), len(wantKinds), runs)
	}
	for i, want := range wantKinds {
		if runs[i].Kind != want {
			t.Fatalf("runs[%d].Kind = %v, want %v", i, runs[i].Kind, want)
		}
	}
	if runs[1].Particles.Emitter != 10 || runs[2].Particles.Emitter != 20 {
		t.Fatalf("particle emitters = (%d, %d), want (10, 20)",
			runs[1].Particles.Emitter, runs[2].Particles.Emitter)
	}
}

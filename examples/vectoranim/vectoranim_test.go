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

func TestGeneratedWorkspaceMatchesStatelessRenderingAcrossFrames(t *testing.T) {
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180})
	renderer := raster.Renderer{
		Width: 320, Height: 180,
		Camera: render.Camera{World: viewport, Viewport: viewport, Aspect: render.AspectFit},
		Clear:  render.RGBA8(16, 24, 39, 255),
	}
	var workspace VectorSceneWorkspace
	for _, elapsed := range []float64{0, 0.18, 0.75, 1.5, 2.16, 3, 0.36} {
		timeline := anim.At(elapsed)
		want := renderer.Render(AppendVectorScene(nil, timeline, viewport, VectorSceneResources{}))
		got := renderer.Render(workspace.Append(timeline, viewport, VectorSceneResources{}))
		if sha256.Sum256(got.Pix) != sha256.Sum256(want.Pix) {
			t.Fatalf("elapsed %g: workspace rendering differs from stateless rendering", elapsed)
		}
	}
}

func TestGeneratedWorkspaceSelectsLODAndReportsFrameMetrics(t *testing.T) {
	if VectorSceneAssetMetrics != (VectorSceneMetrics{
		Nodes: 11, Paths: 3, MaxDepth: 3, CommandCapacity: 8,
	}) {
		t.Fatalf("asset metrics = %+v", VectorSceneAssetMetrics)
	}
	if !VectorSceneAssetMetrics.WithinBudget(VectorSceneBudget{
		MaxNodes: 11, MaxPaths: 3, MaxDepth: 3, MaxCommands: 8,
	}) {
		t.Fatal("asset does not fit its exact budget")
	}
	if VectorSceneAssetMetrics.WithinBudget(VectorSceneBudget{MaxNodes: 10}) {
		t.Fatal("asset unexpectedly fits undersized node budget")
	}

	var workspace VectorSceneWorkspace
	small := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 160, Y: 90})
	commands := workspace.Append(anim.At(3), small, VectorSceneResources{})
	metrics := workspace.FrameMetrics()
	if metrics.LOD != 0 || metrics.Tolerance != 0.5 {
		t.Fatalf("small metrics = %+v, want coarse LOD", metrics)
	}
	if metrics.EmittedCommands != len(commands) {
		t.Fatalf("emitted commands = %d, actual %d", metrics.EmittedCommands, len(commands))
	}

	native := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180})
	workspace.Append(anim.At(3), native, VectorSceneResources{})
	if metrics = workspace.FrameMetrics(); metrics.LOD != 1 || metrics.Tolerance != 0.25 {
		t.Fatalf("native metrics = %+v, want standard LOD", metrics)
	}

	large := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 640, Y: 360})
	workspace.Append(anim.At(3), large, VectorSceneResources{})
	if metrics = workspace.FrameMetrics(); metrics.LOD != 2 || metrics.Tolerance != 0.0625 {
		t.Fatalf("large metrics = %+v, want fine LOD", metrics)
	}
}

func TestGeneratedRuntimeCullsOffViewBoxLeaf(t *testing.T) {
	nodes := []svgNode{
		{
			children:   []uint16{1},
			path:       -1,
			motionPath: -1,
			opacity:    1,
			transform:  math2d.Identity(),
		},
		{
			kind:       1,
			path:       -1,
			motionPath: -1,
			x:          1000,
			y:          1000,
			width:      20,
			height:     20,
			fill:       render.RGBA8(255, 255, 255, 255),
			opacity:    1,
			transform:  math2d.Identity(),
			bounds: math2d.NewRect(
				math2d.Vec2{X: 1000, Y: 1000},
				math2d.Vec2{X: 20, Y: 20},
			),
			hasBounds: true,
		},
	}
	var builder render.Builder
	var metrics svgFrameMetrics
	builder.Reset(nil)
	appendSVGNode(
		&builder,
		nodes,
		0,
		anim.At(0),
		func(uint16) render.FontID { return 0 },
		nil,
		nil,
		0.25,
		math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180}),
		true,
		math2d.Identity(),
		&metrics,
	)
	if commands := builder.Commands(); len(commands) != 0 {
		t.Fatalf("culled leaf emitted %d commands", len(commands))
	}
	if metrics.VisitedNodes != 2 || metrics.CulledNodes != 1 || metrics.EmittedCommands != 0 {
		t.Fatalf("frame metrics = %+v", metrics)
	}
}

func TestManualAccentUsesOnlyPublicCompositionAPIs(t *testing.T) {
	commands := AppendManualAccent(nil, anim.At(2.2))
	if len(commands) != 1 {
		t.Fatalf("manual accent emitted %d commands, want 1", len(commands))
	}
	if _, ok := commands[0].(render.DrawCircle); !ok {
		t.Fatalf("manual accent emitted %T, want render.DrawCircle", commands[0])
	}
}

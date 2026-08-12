package frameplan_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/render/frameplan"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestPlanResolvesScreenAndTargetViewports(t *testing.T) {
	targets := targetLookup(map[render.TextureID]frameplan.TargetInfo{
		1: {W: 80, H: 40, Dynamic: true},
	})
	passes, diagnostics := new(frameplan.Planner).Plan(frameplan.Input{
		Passes:   []render.Pass{{}, {Target: 1}},
		SurfaceW: 320,
		SurfaceH: 180,
		Targets:  targets,
	})
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
	assertViewport(t, passes[0], 320, 180)
	assertViewport(t, passes[1], 80, 40)
}

func TestPlanKeepsExplicitViewport(t *testing.T) {
	want := math2d.NewRect(math2d.Vec2{X: 4, Y: 8}, math2d.Vec2{X: 20, Y: 10})
	passes, _ := new(frameplan.Planner).Plan(frameplan.Input{
		Passes:   []render.Pass{{Camera: render.Camera{Viewport: want}}},
		SurfaceW: 320,
		SurfaceH: 180,
	})
	if got := passes[0].Camera.Viewport; got != want {
		t.Fatalf("viewport = %+v, want %+v", got, want)
	}
}

func TestPlanRejectsDuplicateAndInvalidTargets(t *testing.T) {
	targets := targetLookup(map[render.TextureID]frameplan.TargetInfo{
		1: {W: 80, H: 40, Dynamic: true},
		2: {W: 80, H: 40},
	})
	passes, diagnostics := new(frameplan.Planner).Plan(frameplan.Input{
		Passes:   []render.Pass{{Target: 1}, {Target: 1}, {Target: 2}, {Target: 3}},
		SurfaceW: 320,
		SurfaceH: 180,
		Targets:  targets,
	})
	if len(passes) != 2 {
		t.Fatalf("passes = %d, want target plus base", len(passes))
	}
	want := []frameplan.Diagnostic{
		{Code: frameplan.CodeTargetDuplicate, Target: 1},
		{Code: frameplan.CodeTargetInvalid, Target: 2},
		{Code: frameplan.CodeTargetInvalid, Target: 3},
	}
	if len(diagnostics) != len(want) {
		t.Fatalf("diagnostics = %+v, want %+v", diagnostics, want)
	}
	for index := range want {
		if diagnostics[index] != want[index] {
			t.Fatalf("diagnostics[%d] = %+v, want %+v", index, diagnostics[index], want[index])
		}
	}
}

func TestPlanAppendsMarkedBaseWithPreserve(t *testing.T) {
	for _, tc := range []struct {
		name     string
		passes   []render.Pass
		preserve bool
	}{
		{name: "without screen pass"},
		{name: "with screen pass", passes: []render.Pass{{}}, preserve: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			passes, _ := new(frameplan.Planner).Plan(frameplan.Input{
				Passes: tc.passes, SurfaceW: 320, SurfaceH: 180,
			})
			base := passes[len(passes)-1]
			if !base.Base || base.Preserve != tc.preserve {
				t.Fatalf("base = %+v, want Base and Preserve %v", base, tc.preserve)
			}
		})
	}
}

func TestPlanReuseDoesNotLeakPreviousPasses(t *testing.T) {
	planner := new(frameplan.Planner)
	first, _ := planner.Plan(frameplan.Input{Passes: []render.Pass{{}, {}}, SurfaceW: 10, SurfaceH: 10})
	if len(first) != 3 {
		t.Fatalf("first passes = %d, want 3", len(first))
	}
	second, _ := planner.Plan(frameplan.Input{SurfaceW: 10, SurfaceH: 10})
	if len(second) != 1 || !second[0].Base {
		t.Fatalf("second passes = %+v, want only base", second)
	}
}

func TestPlanCarriesLastBasePostChain(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chains []render.PostChain
		want   int
		has    bool
	}{
		{name: "zero"},
		{name: "one", chains: []render.PostChain{{BelowLayer: 2}}, want: 2, has: true},
		{name: "two", chains: []render.PostChain{{BelowLayer: 2}, {BelowLayer: 7}}, want: 7, has: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commands := make([]render.Command, len(tc.chains))
			for index := range tc.chains {
				commands[index] = tc.chains[index]
			}
			passes, _ := new(frameplan.Planner).Plan(frameplan.Input{
				Commands: commands,
				Passes: []render.Pass{{Target: 1, Commands: []render.Command{
					render.PostChain{BelowLayer: 99},
				}}},
				SurfaceW: 10,
				SurfaceH: 10,
				Targets: targetLookup(map[render.TextureID]frameplan.TargetInfo{
					1: {W: 5, H: 5, Dynamic: true},
				}),
			})
			if passes[0].HasPost {
				t.Fatal("target pass HasPost = true, want false")
			}
			base := passes[len(passes)-1]
			if base.HasPost != tc.has || base.Post.BelowLayer != tc.want {
				t.Fatalf("base post = (%v, %d), want (%v, %d)", base.HasPost, base.Post.BelowLayer, tc.has, tc.want)
			}
		})
	}
}

func TestPostPartitions(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: -1}},
		render.PostChain{BelowLayer: 3},
		render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: 3}},
	}
	below, above := frameplan.PostPartitions(commands, 3)
	if len(below) != 1 || len(above) != 1 || below[0].Layer() != -1 || above[0].Layer() != 3 {
		t.Fatalf("partitions = (%v, %v), want layers -1 and 3", below, above)
	}
	below, above = frameplan.PostPartitions(commands, 0)
	if len(below) != 2 || len(above) != 0 {
		t.Fatalf("full-frame partitions = (%d, %d), want (2, 0)", len(below), len(above))
	}
}

func targetLookup(targets map[render.TextureID]frameplan.TargetInfo) frameplan.TargetLookup {
	return func(id render.TextureID) (frameplan.TargetInfo, bool) {
		info, ok := targets[id]
		return info, ok
	}
}

func assertViewport(t *testing.T, pass frameplan.Pass, width, height int) {
	t.Helper()
	want := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)})
	if pass.Camera.Viewport != want || pass.Width != width || pass.Height != height {
		t.Fatalf("pass = %+v, want viewport %+v and %dx%d", pass, want, width, height)
	}
}

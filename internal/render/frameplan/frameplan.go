// Package frameplan normalizes the pass list every render backend draws.
package frameplan

import (
	"slices"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const (
	// CodeTargetInvalid reports an unknown or non-dynamic pass target.
	CodeTargetInvalid = "render.pass_target_invalid"
	// CodeTargetDuplicate reports a second owner of one target in a frame.
	CodeTargetDuplicate = "render.pass_target_duplicate"
	// CodeTargetUnavailable reports a valid target whose resource cannot render.
	CodeTargetUnavailable = "render.pass_target_unavailable"
	// CodeTargetUpdateConflict reports an upload overwritten by a target pass.
	CodeTargetUpdateConflict = "render.pass_target_update_conflict"
)

// TargetInfo describes one declared texture as a pass target.
type TargetInfo struct {
	W, H    int
	Dynamic bool
}

// TargetLookup resolves texture declarations.
type TargetLookup func(id render.TextureID) (TargetInfo, bool)

// Input contains the frame-level facts used to plan passes.
type Input struct {
	Camera             render.Camera
	Commands           []render.Command
	Passes             []render.Pass
	SurfaceW, SurfaceH int
	Targets            TargetLookup
}

// Pass is one normalized draw pass borrowed from a Planner.
type Pass struct {
	// Pass contains the backend-agnostic camera, commands, target, and viewport.
	render.Pass
	// Base marks the frame's implicit final screen pass.
	Base bool
	// Width is the resolved render-target width.
	Width int
	// Height is the resolved render-target height.
	Height int
	// Preserve reports that an earlier screen pass survives under the base pass.
	Preserve bool
	// Post contains the last post-processing chain declared by the base commands.
	Post render.PostChain
	// HasPost reports whether Post is present, including an empty declared chain.
	HasPost bool
}

// Diagnostic records one pass policy decision for a backend to log.
type Diagnostic struct {
	Code   string
	Target render.TextureID
}

// Message describes the backend-neutral planning decision.
func (d Diagnostic) Message() string {
	switch d.Code {
	case CodeTargetDuplicate:
		return "target already has a pass owner this frame; pass skipped"
	case CodeTargetInvalid:
		return "pass target is not a declared dynamic texture; pass skipped"
	default:
		return "pass target was rejected; pass skipped"
	}
}

// Planner reuses its output slices and is not safe for concurrent use.
type Planner struct {
	passes  []Pass
	diags   []Diagnostic
	claimed []render.TextureID
}

// Plan validates targets, resolves viewports, and appends the base pass.
// Returned slices are borrowed until the next Plan call on this Planner.
func (p *Planner) Plan(in Input) ([]Pass, []Diagnostic) {
	oldPasses := p.passes
	oldDiags := p.diags
	p.passes = p.passes[:0]
	p.diags = p.diags[:0]
	p.claimed = p.claimed[:0]

	surface := bounds(in.SurfaceW, in.SurfaceH)
	preserve := false
	for _, pass := range in.Passes {
		if pass.Target == 0 {
			p.passes = append(p.passes, Pass{
				Pass:   pass.ResolveViewport(surface),
				Width:  in.SurfaceW,
				Height: in.SurfaceH,
			})
			preserve = true
			continue
		}
		if slices.Contains(p.claimed, pass.Target) {
			p.diags = append(p.diags, Diagnostic{Code: CodeTargetDuplicate, Target: pass.Target})
			continue
		}
		info, ok := lookup(in.Targets, pass.Target)
		if !ok || !info.Dynamic {
			p.diags = append(p.diags, Diagnostic{Code: CodeTargetInvalid, Target: pass.Target})
			continue
		}
		p.claimed = append(p.claimed, pass.Target)
		p.passes = append(p.passes, Pass{
			Pass:   pass.ResolveViewport(bounds(info.W, info.H)),
			Width:  info.W,
			Height: info.H,
		})
	}

	base := Pass{
		Pass: render.Pass{
			Camera:   in.Camera,
			Commands: in.Commands,
		}.ResolveViewport(surface),
		Base:     true,
		Width:    in.SurfaceW,
		Height:   in.SurfaceH,
		Preserve: preserve,
	}
	base.Post, base.HasPost = lastPostChain(in.Commands)
	p.passes = append(p.passes, base)

	clearTail(oldPasses, len(p.passes))
	clearTail(oldDiags, len(p.diags))
	return p.passes, p.diags
}

// PostPartitions splits commands around belowLayer and drops PostChain commands.
func PostPartitions(commands []render.Command, belowLayer int) (below, above []render.Command) {
	for _, command := range commands {
		if _, ok := command.(render.PostChain); ok {
			continue
		}
		if belowLayer == 0 || command.Layer() < belowLayer {
			below = append(below, command)
			continue
		}
		above = append(above, command)
	}
	return below, above
}

func bounds(width, height int) math2d.Rect {
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)})
}

func lookup(targets TargetLookup, id render.TextureID) (TargetInfo, bool) {
	if targets == nil {
		return TargetInfo{}, false
	}
	return targets(id)
}

func lastPostChain(commands []render.Command) (render.PostChain, bool) {
	var chain render.PostChain
	found := false
	for _, command := range commands {
		if candidate, ok := command.(render.PostChain); ok {
			chain = candidate
			found = true
		}
	}
	return chain, found
}

func clearTail[S ~[]E, E any](slice S, length int) {
	if length < len(slice) {
		clear(slice[length:])
	}
}

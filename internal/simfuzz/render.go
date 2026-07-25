package simfuzz

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// Render layers, drawn back to front.
const (
	fuzzLayerBounds = 0
	fuzzLayerEntity = 10
	fuzzLayerVector = 20
)

// markerRadius is the size of the dot drawn for an entity with no collider to
// take a size from.
const markerRadius = 2.0

// velocityScale converts a velocity into a line length, and velocityMaxLength
// caps it: input-driven velocities reach maxInputVelocity, and an unclamped
// line would stretch far outside the bounds and hide everything else.
const (
	velocityScale     = 0.25
	velocityMaxLength = 60.0
)

var (
	colorBounds  = render.RGBA(0.35, 0.35, 0.42, 1)
	colorStatic  = render.RGBA(0.45, 0.45, 0.50, 1)
	colorDynamic = render.RGBA(0.30, 0.70, 0.90, 1)
	colorPlain   = render.RGBA(0.85, 0.70, 0.25, 1)
	colorVector  = render.RGBA(0.95, 0.45, 0.35, 1)
)

// Render draws the fuzz world so a recorded case can be watched instead of only
// being diffed as a checksum.
//
// It costs the fuzzer nothing: the targets drive Driver.Step, and Render is
// called only from Driver.Frame, so this code never runs during a fuzz run. It
// exists for replaying a case through a host.
//
// Entities are drawn from their colliders rather than their sprites, because
// most fuzz entities have no sprite — the script decides which components
// exist, and a case is far easier to read when every live entity is visible.
// Colour encodes what an entity carries, which is usually the question being
// asked of a failing case.
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	cmds := ctx.World.RenderCommands()
	cmds = append(cmds, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: fuzzLayerBounds},
		Rect:        fuzzBounds,
		Stroke:      colorBounds,
		StrokeWidth: 1,
	})

	for _, id := range g.trackedIDs() {
		transform, ok := ctx.World.Transform(id)
		if !ok {
			// Despawned: P10 removes entities mid-run.
			continue
		}
		// A non-finite position cannot be drawn meaningfully, and feeding one
		// to a backend is its own hazard. The numeric scan in PostStep is what
		// reports these; skipping here keeps the rest of the frame readable.
		if !finiteVec(transform.Position) {
			continue
		}

		cmds = append(cmds, entityCommand(ctx.World, id, transform.Position))
		if line, ok := velocityCommand(ctx.World, id, transform.Position); ok {
			cmds = append(cmds, line)
		}
	}
	return cmds
}

// entityCommand draws one entity shaped like its collider, coloured by whether
// it is static, dynamic, or carries no Body at all.
func entityCommand(view scene.WorldView, id scene.EntityID, position math2d.Vec2) render.Command {
	base := render.BaseCommand{LayerValue: fuzzLayerEntity}
	fill := colorPlain
	if body, ok := view.Body(id); ok {
		fill = colorDynamic
		if body.InvMass == 0 {
			fill = colorStatic
		}
	}

	if collider, ok := view.RectCollider(id); ok && finiteVec(collider.HalfExtends) {
		return render.DrawRect{
			BaseCommand: base,
			Rect: math2d.NewRect(
				position.Sub(collider.HalfExtends),
				position.Add(collider.HalfExtends),
			),
			Fill: fill,
		}
	}

	radius := markerRadius
	if collider, ok := view.CircleCollider(id); ok && isFinite(collider.Radius) && collider.Radius > 0 {
		radius = collider.Radius
	}
	return render.DrawCircle{
		BaseCommand: base,
		Center:      position,
		Radius:      radius,
		Fill:        fill,
	}
}

// velocityCommand draws a capped line along an entity's velocity. The cap
// matters: a fuzzed velocity reaches maxInputVelocity, and an unclamped line
// would cover the whole viewport.
func velocityCommand(view scene.WorldView, id scene.EntityID, position math2d.Vec2) (render.Command, bool) {
	velocity, ok := view.Velocity(id)
	if !ok || !finiteVec(velocity.Linear) {
		return nil, false
	}
	offset := velocity.Linear.Mul(velocityScale)
	if offset.X == 0 && offset.Y == 0 {
		return nil, false
	}
	if length := offset.Len(); length > velocityMaxLength {
		offset = offset.Mul(velocityMaxLength / length)
	}
	return render.DrawLine{
		BaseCommand: render.BaseCommand{LayerValue: fuzzLayerVector},
		From:        position,
		To:          position.Add(offset),
		Color:       colorVector,
		Width:       0.6,
	}, true
}

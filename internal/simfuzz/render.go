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
	fuzzLayerShadow = 5
	fuzzLayerEntity = 10
	fuzzLayerVector = 20
)

// markerRadius is the size of the outline drawn for an entity with no collider
// to take a size from; markerStrokeWidth keeps that outline thin enough not to
// swallow a small entity.
const (
	markerRadius      = 2.0
	markerStrokeWidth = 0.5
)

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

// fuzzProjection is the 2.5D projection the fuzz world runs with. Setting one
// is what makes Height visible at all: the automatic render path lifts a
// sprite by its Z and drops a ground shadow only when the world has a
// projection, and it is otherwise drawn flat. Markers for entities without a
// sprite are lifted through the same projection so a ballistic case reads the
// same way whether or not the script gave the entity a sprite.
var fuzzProjection = render.Projection{
	LiftPerZ:         1,
	ScalePerZ:        0.01,
	ShadowScale:      math2d.Vec2{X: 1, Y: 0.5},
	ShadowAlpha:      0.35,
	ShadowShrinkPerZ: 0.02,
	ShadowFadePerZ:   0.01,
}

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
		// The marker is an outline and is always drawn, even when the entity
		// has a sprite. A fuzzed sprite is frequently degenerate — a seed
		// zero-fills its bytes, giving radius 0 and alpha 0 — so treating the
		// presence of a sprite as "already visible" would make entities
		// disappear. An outline never hides the sprite it wraps.
		position := transform.Position
		if z, ok := entityHeight(ctx.World, id); ok {
			radius := markerFootprint(ctx.World, id)
			projected := fuzzProjection.Project(position, math2d.Vec2{X: radius, Y: radius}, z)
			// Only shadow entities the automatic path did not: a sprite
			// already gets one from the same projection.
			if projected.ShadowAlpha > 0 && !hasSprite(ctx.World, id) {
				cmds = append(cmds, render.DrawEllipse{
					BaseCommand: render.BaseCommand{LayerValue: fuzzLayerShadow},
					Center:      position.Add(projected.ShadowCenter),
					RadiusX:     projected.ShadowRadii.X,
					RadiusY:     projected.ShadowRadii.Y,
					Fill:        render.RGBA(0, 0, 0, projected.ShadowAlpha),
				})
			}
			position = projected.BodyPosition
		}

		cmds = append(cmds, entityCommand(ctx.World, id, position))
		if line, ok := velocityCommand(ctx.World, id, position); ok {
			cmds = append(cmds, line)
		}
	}
	return cmds
}

// hasSprite reports whether the automatic render path already draws this
// entity's body.
func hasSprite(view scene.WorldView, id scene.EntityID) bool {
	if _, ok := view.CircleSprite(id); ok {
		return true
	}
	if _, ok := view.RectSprite(id); ok {
		return true
	}
	_, ok := view.ImageSprite(id)
	return ok
}

// entityHeight returns a drawable Z for an entity, if it has one off the
// ground. Reading it needs WorldView.Height, which the view gained for exactly
// this kind of consumer.
func entityHeight(view scene.WorldView, id scene.EntityID) (float64, bool) {
	height, ok := view.Height(id)
	if !ok || !isFinite(height.Z) || height.Z <= 0 {
		return 0, false
	}
	return height.Z, true
}

// markerFootprint is the half-extent the projection uses to size the shadow of
// a marker-drawn entity.
func markerFootprint(view scene.WorldView, id scene.EntityID) float64 {
	if collider, ok := view.CircleCollider(id); ok && isFinite(collider.Radius) && collider.Radius > 0 {
		return collider.Radius
	}
	if collider, ok := view.RectCollider(id); ok && finiteVec(collider.HalfExtends) {
		if collider.HalfExtends.X > 0 {
			return collider.HalfExtends.X
		}
	}
	return markerRadius
}

// entityCommand outlines one entity in the shape of its collider, coloured by
// whether it is static, dynamic, or carries no Body at all.
func entityCommand(view scene.WorldView, id scene.EntityID, position math2d.Vec2) render.Command {
	base := render.BaseCommand{LayerValue: fuzzLayerEntity}
	stroke := colorPlain
	if body, ok := view.Body(id); ok {
		stroke = colorDynamic
		if body.InvMass == 0 {
			stroke = colorStatic
		}
	}

	if collider, ok := view.RectCollider(id); ok && finiteVec(collider.HalfExtends) {
		return render.DrawRect{
			BaseCommand: base,
			Rect: math2d.NewRect(
				position.Sub(collider.HalfExtends),
				position.Add(collider.HalfExtends),
			),
			Stroke:      stroke,
			StrokeWidth: markerStrokeWidth,
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
		Stroke:      stroke,
		StrokeWidth: markerStrokeWidth,
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

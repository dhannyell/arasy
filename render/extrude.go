package render

import "github.com/dhannyell/arasy/math2d"

// This file is the projection's authoring vocabulary for fake volume. The
// engine has no vertical faces -- everything drawn lives on the ground plane --
// but a wall can be faked by sweeping a ground edge along the direction height
// moves things, and a box by capping two such walls with a lifted top. These
// helpers do that sweep so a game never has to derive the lift arithmetic
// itself.

// LiftVector is how far, in world units, one unit of height moves a body under
// this projection: the resolved LiftAxis times LiftPerZ. It is the single
// vector every piece of hand-authored volume should extrude along, so authored
// geometry and engine-lifted entities can never disagree about which way up is.
//
// Under a plain 2D camera it is zero: height has nowhere on screen to go.
func (p Projection) LiftVector() math2d.Vec2 {
	return p.liftAxis().Mul(p.LiftPerZ)
}

// Lift places a ground point at height z: the point moved z lift-vectors up.
func (p Projection) Lift(ground math2d.Vec2, z float64) math2d.Vec2 {
	return ground.Add(p.LiftVector().Mul(z))
}

// LiftRect places a ground rectangle at height z, keeping its size: the roof
// of a box whose floor is rect.
func (p Projection) LiftRect(rect math2d.Rect, z float64) math2d.Rect {
	return math2d.NewRect(p.Lift(rect.Min, z), rect.Size())
}

// Wall is one vertical face: the ground edge from a to b, swept from height z0
// up to height z1. It comes out a parallelogram on screen because DrawPolygon
// walks its vertices through the camera, view and all.
func (p Projection) Wall(a, b math2d.Vec2, z0, z1 float64, fill Color, layer int) DrawPolygon {
	return DrawPolygon{
		Points: []math2d.Vec2{
			p.Lift(a, z1),
			p.Lift(b, z1),
			p.Lift(b, z0),
			p.Lift(a, z0),
		},
		Fill:        fill,
		BaseCommand: BaseCommand{LayerValue: layer},
	}
}

// BlockStyle colors the parts of an extruded block. Giving the two walls
// different tones is what makes the box read as volume: same-colored walls
// merge into one flat shape.
type BlockStyle struct {
	Top Color

	// WallX is the wall along the block's X boundary (the left or right side,
	// whichever the projection makes visible). WallY is the one along its Y
	// boundary. Under the classic 2:1 isometric view WallX faces down-right
	// and WallY faces down-left.
	WallX Color
	WallY Color
}

// ExtrudeRect appends a box to dst: the ground rectangle rect raised from
// height z0 to z1, drawn as the walls the projection makes visible plus the
// lifted top. Which walls those are follows from the lift vector -- a face is
// visible when the viewer is on its side of the block, and the viewer sits
// opposite to where height moves things.
//
// A projection that does not lift (a plain 2D camera) emits only the top,
// which is the honest answer: with no height on screen there is no side to
// see.
func (p Projection) ExtrudeRect(dst []Command, rect math2d.Rect, z0, z1 float64, style BlockStyle, layer int) []Command {
	lift := p.LiftVector()

	if lift.X < 0 {
		dst = append(dst, p.Wall(math2d.Vec2{X: rect.Max.X, Y: rect.Min.Y}, rect.Max, z0, z1, style.WallX, layer))
	} else if lift.X > 0 {
		dst = append(dst, p.Wall(rect.Min, math2d.Vec2{X: rect.Min.X, Y: rect.Max.Y}, z0, z1, style.WallX, layer))
	}

	if lift.Y < 0 {
		dst = append(dst, p.Wall(math2d.Vec2{X: rect.Min.X, Y: rect.Max.Y}, rect.Max, z0, z1, style.WallY, layer))
	} else if lift.Y > 0 {
		dst = append(dst, p.Wall(rect.Min, math2d.Vec2{X: rect.Max.X, Y: rect.Min.Y}, z0, z1, style.WallY, layer))
	}

	return append(dst, DrawRect{
		Rect:        p.LiftRect(rect, z1),
		Fill:        style.Top,
		BaseCommand: BaseCommand{LayerValue: layer},
	})
}

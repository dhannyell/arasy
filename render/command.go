package render

import "github.com/dhannyell/frameforge/math2d"

// Command represents a render operation.
type Command interface {
	isRenderCommand()
}

// DrawRect describes a filled rectangle draw command.
type DrawRect struct {
	Rect        math2d.Rect
	Fill        Color
	Stroke      Color
	StrokeWidth float64
	Layer       int
}

// DrawCircle describes a filled circle draw command.
type DrawCircle struct {
	Center      math2d.Vec2
	Radius      float64
	Fill        Color
	Stroke      Color
	StrokeWidth float64
	Layer       int
}

// DrawLine describes a line draw command.
type DrawLine struct {
	From  math2d.Vec2
	To    math2d.Vec2
	Width float64
	Color Color
	Layer int
}

// DrawText describes a text draw command.
type DrawText struct {
	Position math2d.Vec2
	Text     string
	Color    Color
	Size     float64
	Layer    int
}

// DrawArc describes an arc stroke draw command.
type DrawArc struct {
	Center     math2d.Vec2
	Radius     float64
	StartAngle float64
	EndAngle   float64
	Width      float64
	Color      Color
	Layer      int
}

func (DrawRect) isRenderCommand()   {}
func (DrawCircle) isRenderCommand() {}
func (DrawLine) isRenderCommand()   {}
func (DrawText) isRenderCommand()   {}
func (DrawArc) isRenderCommand()    {}

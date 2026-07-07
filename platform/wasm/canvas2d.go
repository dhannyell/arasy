//go:build js && wasm

package wasm

import (
	"fmt"
	"math"
	"syscall/js"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

type Canvas2D struct {
	ctx    js.Value
	canvas js.Value
	clear  render.Color
}

func newCanvas2D(canvas js.Value, clear render.Color) *Canvas2D {
	return &Canvas2D{
		ctx:    canvas.Call("getContext", "2d"),
		canvas: canvas,
		clear:  clear,
	}
}

func (c *Canvas2D) Render(frame runtime.Frame) {
	w := c.canvas.Get("width").Float()
	h := c.canvas.Get("height").Float()
	c.ctx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	c.ctx.Call("clearRect", 0, 0, w, h)
	if c.clear.A > 0 {
		c.ctx.Set("fillStyle", cssColor(c.clear))
		c.ctx.Call("fillRect", 0, 0, w, h)
	}

	for _, cmd := range frame.Commands {
		c.ctx.Call("save")
		c.applyClip(cmd, frame.Camera)
		switch cmd := cmd.(type) {
		case render.DrawRect:
			c.drawRect(cmd, frame.Camera)
		case render.DrawCircle:
			c.drawCircle(cmd, frame.Camera)
		case render.DrawPoint:
			c.drawPoint(cmd, frame.Camera)
		case render.DrawLine:
			c.drawLine(cmd, frame.Camera)
		case render.DrawArc:
			c.drawArc(cmd, frame.Camera)
		case render.DrawText:
			c.drawText(cmd, frame.Camera)
		case render.DrawPolygon:
			c.drawPolygon(cmd, frame.Camera)
		}
		c.ctx.Call("restore")
	}
}

func cssColor(c render.Color) string {
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", int(math.Round(c.R*255)), int(math.Round(c.G*255)), int(math.Round(c.B*255)), c.A)
}

func (c *Canvas2D) drawRect(cmd render.DrawRect, camera render.Camera) {
	x, y, w, h := screenRect(cmd.Rect, camera)

	if cmd.CornerRadius > 0 && c.ctx.Get("roundRect").Truthy() {
		c.ctx.Call("beginPath")
		c.ctx.Call("roundRect", x, y, w, h, camera.WorldScalarToScreen(cmd.CornerRadius))
		if cmd.Fill.A > 0 {
			c.ctx.Set("fillStyle", cssColor(cmd.Fill))
			c.ctx.Call("fill")
		}
		if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
			c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
			c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
			c.ctx.Call("stroke")
		}
		return
	}

	if cmd.Fill.A > 0 {
		c.ctx.Set("fillStyle", cssColor(cmd.Fill))
		c.ctx.Call("fillRect", x, y, w, h)
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
		c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
		c.ctx.Call("strokeRect", x, y, w, h)
	}
}

func (c *Canvas2D) drawCircle(cmd render.DrawCircle, camera render.Camera) {
	center := camera.WorldToScreen(cmd.Center)
	radius := camera.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("arc", center.X, center.Y, radius, 0, 2*math.Pi)
	if cmd.Fill.A > 0 {
		c.ctx.Set("fillStyle", cssColor(cmd.Fill))
		c.ctx.Call("fill")
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
		c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
		c.ctx.Call("stroke")
	}
}

func (c *Canvas2D) drawPoint(cmd render.DrawPoint, camera render.Camera) {
	if cmd.Color.A <= 0 {
		return
	}

	center := camera.WorldToScreen(cmd.Center)
	radius := camera.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("arc", center.X, center.Y, radius, 0, 2*math.Pi)
	c.ctx.Set("fillStyle", cssColor(cmd.Color))
	c.ctx.Call("fill")
}

func (c *Canvas2D) drawLine(cmd render.DrawLine, camera render.Camera) {
	if cmd.Color.A <= 0 || cmd.Width <= 0 {
		return
	}

	from := camera.WorldToScreen(cmd.From)
	to := camera.WorldToScreen(cmd.To)

	c.ctx.Call("beginPath")
	c.ctx.Call("moveTo", from.X, from.Y)
	c.ctx.Call("lineTo", to.X, to.Y)
	c.ctx.Set("strokeStyle", cssColor(cmd.Color))
	c.ctx.Set("lineWidth", math.Max(1, camera.WorldScalarToScreen(cmd.Width)))
	c.ctx.Call("stroke")
}

func (c *Canvas2D) drawArc(cmd render.DrawArc, camera render.Camera) {
	if cmd.Color.A <= 0 || cmd.Width <= 0 {
		return
	}

	center := camera.WorldToScreen(cmd.Center)
	radius := camera.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("arc", center.X, center.Y, radius, cmd.StartAngle, cmd.EndAngle)
	c.ctx.Set("strokeStyle", cssColor(cmd.Color))
	c.ctx.Set("lineWidth", math.Max(1, camera.WorldScalarToScreen(cmd.Width)))
	c.ctx.Call("stroke")
}

func (c *Canvas2D) drawText(cmd render.DrawText, camera render.Camera) {
	if cmd.Text == "" || cmd.Color.A <= 0 {
		return
	}

	size := cmd.Size
	if size <= 0 {
		size = 13
	}
	screenSize := camera.WorldScalarToScreen(size)
	if screenSize <= 0 {
		return
	}

	pos := camera.WorldToScreen(cmd.Position)
	c.ctx.Set("fillStyle", cssColor(cmd.Color))
	c.ctx.Set("font", fmt.Sprintf("%gpx monospace", screenSize))
	c.ctx.Set("textAlign", canvasTextAlign(cmd.Align))
	c.ctx.Set("textBaseline", canvasTextBaseline(cmd.Baseline))

	if cmd.Rotation == 0 {
		c.ctx.Call("fillText", cmd.Text, pos.X, pos.Y)
		return
	}

	c.ctx.Call("translate", pos.X, pos.Y)
	c.ctx.Call("rotate", cmd.Rotation)
	c.ctx.Call("fillText", cmd.Text, 0, 0)
}

func (c *Canvas2D) drawPolygon(cmd render.DrawPolygon, camera render.Camera) {
	if len(cmd.Points) < 3 {
		return
	}

	first := camera.WorldToScreen(cmd.Points[0])
	c.ctx.Call("beginPath")
	c.ctx.Call("moveTo", first.X, first.Y)
	for _, point := range cmd.Points[1:] {
		screen := camera.WorldToScreen(point)
		c.ctx.Call("lineTo", screen.X, screen.Y)
	}
	c.ctx.Call("closePath")

	if cmd.Fill.A > 0 {
		c.ctx.Set("fillStyle", cssColor(cmd.Fill))
		c.ctx.Call("fill")
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
		c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
		c.ctx.Call("stroke")
	}
}

func (c *Canvas2D) applyClip(cmd render.Command, camera render.Camera) {
	clip := commandClip(cmd)
	if clip.Width() <= 0 || clip.Height() <= 0 {
		return
	}

	cMin := camera.WorldToScreen(clip.Min)
	cMax := camera.WorldToScreen(clip.Max)

	x := math.Min(cMin.X, cMax.X)
	y := math.Min(cMin.Y, cMax.Y)
	w := math.Abs(cMax.X - cMin.X)
	h := math.Abs(cMax.Y - cMin.Y)

	if w <= 0 || h <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("rect", x, y, w, h)
	c.ctx.Call("clip")
}

func commandClip(cmd render.Command) math2d.Rect {
	switch cm := cmd.(type) {
	case render.DrawRect:
		return cm.Clip
	case render.DrawCircle:
		return cm.Clip
	case render.DrawPoint:
		return cm.Clip
	case render.DrawLine:
		return cm.Clip
	case render.DrawText:
		return cm.Clip
	case render.DrawArc:
		return cm.Clip
	case render.DrawPolygon:
		return cm.Clip
	default:
		return math2d.Rect{}
	}
}

func screenRect(rect math2d.Rect, camera render.Camera) (x, y, w, h float64) {
	min := camera.WorldToScreen(rect.Min)
	max := camera.WorldToScreen(rect.Max)

	x = math.Min(min.X, max.X)
	y = math.Min(min.Y, max.Y)
	w = math.Abs(max.X - min.X)
	h = math.Abs(max.Y - min.Y)
	return x, y, w, h
}

func canvasTextAlign(align render.TextAlign) string {
	switch align {
	case render.AlignCenter:
		return "center"
	case render.AlignRight:
		return "right"
	default:
		return "left"
	}
}

func canvasTextBaseline(baseline render.TextBaseline) string {
	switch baseline {
	case render.BaselineTop:
		return "top"
	case render.BaselineMiddle:
		return "middle"
	case render.BaselineBottom:
		return "bottom"
	default:
		return "alphabetic"
	}
}

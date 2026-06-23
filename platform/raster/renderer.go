package raster

import (
	"image"
	"image/color"
	"sort"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Renderer draws backend-agnostic render commands into an RGBA image.
type Renderer struct {
	Width, Height int
	Camera        render.Camera
	Clear         render.Color
}

// Render draws commands into a new RGBA image.
func (r Renderer) Render(commands []render.Command) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))

	fillRect(img, img.Bounds(), toRGBA(r.Clear))

	orderedCommands := append([]render.Command(nil), commands...)
	sort.SliceStable(orderedCommands, func(i, j int) bool {
		return commandLayer(orderedCommands[i]) < commandLayer(orderedCommands[j])
	})

	for _, command := range orderedCommands {
		switch command := command.(type) {
		case render.DrawRect:
			r.drawRect(img, command)
		case render.DrawCircle:
			r.drawCircle(img, command)
		case render.DrawLine:
			r.drawLine(img, command)
		}
	}

	return img
}

func (r Renderer) drawRect(img *image.RGBA, command render.DrawRect) {
	screenMin := r.Camera.WorldToScreen(command.Rect.Min)
	screenMax := r.Camera.WorldToScreen(command.Rect.Max)

	rect := image.Rect(
		int(screenMin.X),
		int(screenMin.Y),
		int(screenMax.X),
		int(screenMax.Y),
	)

	fillRect(img, rect, toRGBA(command.Fill))
}

func (r Renderer) drawCircle(img *image.RGBA, command render.DrawCircle) {
	center := r.Camera.WorldToScreen(command.Center)

	worldSize := r.Camera.World.Size()
	viewportSize := r.Camera.Viewport.Size()
	radius := command.Radius * (viewportSize.X / worldSize.X)

	fillCircle(img, center, radius, toRGBA(command.Fill))
}

func (r Renderer) drawLine(img *image.RGBA, command render.DrawLine) {
	from := r.Camera.WorldToScreen(command.From)
	to := r.Camera.WorldToScreen(command.To)

	worldSize := r.Camera.World.Size()
	viewportSize := r.Camera.Viewport.Size()
	width := command.Width * (viewportSize.X / worldSize.X)

	drawLine(img, from, to, width, toRGBA(command.Color))
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	bounds := rect.Intersect(img.Bounds())

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawLine(img *image.RGBA, from, to math2d.Vec2, width float64, c color.RGBA) {
	if width <= 0 {
		return
	}

	line := to.Sub(from)
	length := line.Len()
	if length == 0 {
		fillCircle(img, from, width*0.5, c)
		return
	}

	radius := width * 0.5
	minX := int(minFloat(from.X, to.X) - radius)
	maxX := int(maxFloat(from.X, to.X) + radius)
	minY := int(minFloat(from.Y, to.Y) - radius)
	maxY := int(maxFloat(from.Y, to.Y) + radius)

	bounds := image.Rect(minX, minY, maxX+1, maxY+1).Intersect(img.Bounds())
	lengthSq := line.LenSq()
	radiusSq := radius * radius

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			point := math2d.Vec2{
				X: float64(x) + 0.5,
				Y: float64(y) + 0.5,
			}

			t := point.Sub(from).Dot(line) / lengthSq
			if t < 0 || t > 1 {
				continue
			}

			closest := from.Lerp(to, t)
			if point.DistanceSq(closest) <= radiusSq {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillCircle(img *image.RGBA, center math2d.Vec2, radius float64, c color.RGBA) {
	minX := int(center.X - radius)
	maxX := int(center.X + radius)
	minY := int(center.Y - radius)
	maxY := int(center.Y + radius)

	radiusSq := radius * radius
	bounds := image.Rect(minX, minY, maxX+1, maxY+1).Intersect(img.Bounds())

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y

			if dx*dx+dy*dy <= radiusSq {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}

	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}

	return b
}

func toRGBA(c render.Color) color.RGBA {
	return color.RGBA{
		R: toByte(c.R),
		G: toByte(c.G),
		B: toByte(c.B),
		A: toByte(c.A),
	}
}

func toByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}

	if v >= 1 {
		return 255
	}

	return uint8(v*255 + 0.5)
}

func commandLayer(command render.Command) int {
	switch command := command.(type) {
	case render.DrawRect:
		return command.Layer
	case render.DrawCircle:
		return command.Layer
	case render.DrawLine:
		return command.Layer
	case render.DrawText:
		return command.Layer
	default:
		return 0
	}
}

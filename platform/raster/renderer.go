package raster

import (
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"math"
	"sort"
	"sync"

	"github.com/dhannyell/frameforge/asset/pixel"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

var missingTextureColor = color.RGBA{R: 255, G: 0, B: 255, A: 255}

type tintFactors struct {
	r, g, b, a float64
}

var noTint = tintFactors{1, 1, 1, 1}

// Renderer draws backend-agnostic render commands into an RGBA image.
type Renderer struct {
	Width, Height int
	Camera        render.Camera
	Clear         render.Color
	// Textures resolves TextureIDs to decoded pixels. Nil is valid:
	// every DrawImage then renders the missing-texture placeholder.
	Textures pixel.Registry
}

// Render draws commands into a new RGBA image.
func (r Renderer) Render(commands []render.Command) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))

	fillRect(img, img.Bounds(), toRGBA(r.Clear))

	orderedCommands := append([]render.Command(nil), commands...)
	sort.SliceStable(orderedCommands, func(i, j int) bool {
		return orderedCommands[i].Layer() < orderedCommands[j].Layer()
	})

	for _, command := range orderedCommands {
		switch command := command.(type) {
		case render.DrawRect:
			r.drawRect(img, command)
		case render.DrawCircle:
			r.drawCircle(img, command)
		case render.DrawPoint:
			r.drawPoint(img, command)
		case render.DrawLine:
			r.drawLine(img, command)
		case render.DrawArc:
			r.drawArc(img, command)
		case render.DrawText:
			r.drawText(img, command)
		case render.DrawPolygon:
			r.drawPolygon(img, command)
		case render.DrawImage:
			r.drawImage(img, command)
		case render.DrawImageQuad:
			r.drawImageQuad(img, command)
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
	bounds := r.clipBounds(img, rect, command.Clip)

	if command.Fill.A > 0 {
		if command.CornerRadius > 0 {
			fillRoundedRect(img, rect, r.Camera.WorldScalarToScreen(command.CornerRadius), bounds, toRGBA(command.Fill))
		} else {
			fillRect(img, rect.Intersect(bounds), toRGBA(command.Fill))
		}
	}

	if command.Stroke.A > 0 && command.StrokeWidth > 0 {
		strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
		if command.CornerRadius > 0 {
			strokeRoundedRect(img, rect, r.Camera.WorldScalarToScreen(command.CornerRadius), strokeWidth, bounds, toRGBA(command.Stroke))
		} else {
			strokeRect(img, rect, int(math.Ceil(strokeWidth)), bounds, toRGBA(command.Stroke))
		}
	}
}

func (r Renderer) drawCircle(img *image.RGBA, command render.DrawCircle) {
	center := r.Camera.WorldToScreen(command.Center)

	radius := r.Camera.WorldScalarToScreen(command.Radius)
	bounds := r.clipBounds(img, circleBounds(center, radius), command.Clip)

	if command.Fill.A > 0 {
		fillCircle(img, center, radius, bounds, toRGBA(command.Fill))
	}

	if command.Stroke.A > 0 && command.StrokeWidth > 0 {
		strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
		strokeCircle(img, center, radius, strokeWidth, bounds, toRGBA(command.Stroke))
	}
}

func (r Renderer) drawPoint(img *image.RGBA, command render.DrawPoint) {
	center := r.Camera.WorldToScreen(command.Center)
	radius := r.Camera.WorldScalarToScreen(command.Radius)
	bounds := r.clipBounds(img, circleBounds(center, radius), command.Clip)

	fillCircle(img, center, radius, bounds, toRGBA(command.Color))
}

func (r Renderer) drawLine(img *image.RGBA, command render.DrawLine) {
	from := r.Camera.WorldToScreen(command.From)
	to := r.Camera.WorldToScreen(command.To)

	width := r.Camera.WorldScalarToScreen(command.Width)
	bounds := r.clipBounds(img, lineBounds(from, to, width), command.Clip)

	drawLine(img, from, to, width, bounds, toRGBA(command.Color))
}

func drawLine(img *image.RGBA, from, to math2d.Vec2, width float64, bounds image.Rectangle, c color.RGBA) {
	if width <= 0 {
		return
	}

	line := to.Sub(from)
	length := line.Len()
	if length == 0 {
		fillCircle(img, from, width*0.5, bounds, c)
		return
	}

	radius := width * 0.5
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
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func (r Renderer) drawArc(img *image.RGBA, command render.DrawArc) {
	center := r.Camera.WorldToScreen(command.Center)

	radius := r.Camera.WorldScalarToScreen(command.Radius)

	width := r.Camera.WorldScalarToScreen(command.Width)
	bounds := r.clipBounds(img, circleBounds(center, radius), command.Clip)

	strokeArc(img, center, radius, width, command.StartAngle, command.EndAngle, bounds, toRGBA(command.Color))
}

// textFace is the built-in bitmap font used to render DrawText commands.
// Its native pixel height anchors the scale factor derived from
// DrawText.Size, so text grows/shrinks with the camera like other shapes.
var textFace = basicfont.Face7x13

func (r Renderer) drawText(img *image.RGBA, command render.DrawText) {
	if command.Text == "" || command.Color.A <= 0 {
		return
	}

	metrics := textFace.Metrics()
	nativeHeight := float64(metrics.Height.Ceil())
	nativeAscent := float64(metrics.Ascent.Ceil())
	nativeWidth := float64(font.MeasureString(textFace, command.Text).Ceil())
	if nativeWidth <= 0 {
		return
	}

	size := command.Size
	if size <= 0 {
		size = nativeHeight
	}

	measured := render.MeasureText(command.Text, size)
	destWidth := r.Camera.WorldScalarToScreen(measured.X)
	destHeight := r.Camera.WorldScalarToScreen(measured.Y)
	if destWidth <= 0 || destHeight <= 0 {
		return
	}

	glyphs := image.NewRGBA(image.Rect(0, 0, int(math.Ceil(nativeWidth)), int(math.Ceil(nativeHeight))))
	drawer := &font.Drawer{
		Dst:  glyphs,
		Src:  image.NewUniform(toRGBA(command.Color)),
		Face: textFace,
		Dot:  fixed.Point26_6{X: 0, Y: metrics.Ascent},
	}
	drawer.DrawString(command.Text)

	screenPos := r.Camera.WorldToScreen(command.Position)

	offsetX := 0.0
	switch command.Align {
	case render.AlignCenter:
		offsetX = -destWidth / 2
	case render.AlignRight:
		offsetX = -destWidth
	}

	offsetY := 0.0
	switch command.Baseline {
	case render.BaselineTop:
		offsetY = 0
	case render.BaselineMiddle:
		offsetY = -destHeight / 2
	case render.BaselineBottom:
		offsetY = -destHeight
	default: // BaselineAlphabetic
		offsetY = -(nativeAscent / nativeHeight) * destHeight
	}

	destX := screenPos.X + offsetX
	destY := screenPos.Y + offsetY
	bounds := r.clipBounds(img, rotatedRectBounds(screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation), command.Clip)

	if command.Rotation == 0 {
		blitScaled(img, glyphs, glyphs.Bounds(), destX, destY, destWidth, destHeight, bounds, noTint)
		return
	}

	blitScaledRotated(img, glyphs, glyphs.Bounds(), screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation, bounds, noTint)
}

func (r Renderer) drawPolygon(img *image.RGBA, command render.DrawPolygon) {
	if len(command.Points) < 3 {
		return
	}

	points := make([]math2d.Vec2, len(command.Points))
	for i, point := range command.Points {
		points[i] = r.Camera.WorldToScreen(point)
	}

	if command.Fill.A > 0 {
		fillPolygon(img, points, r.clipBounds(img, polygonBounds(points), command.Clip), toRGBA(command.Fill))
	}

	strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
	if command.Stroke.A <= 0 || strokeWidth <= 0 {
		return
	}

	for i := range points {
		from := points[i]
		to := points[(i+1)%len(points)]
		drawLine(img, from, to, strokeWidth, r.clipBounds(img, lineBounds(from, to, strokeWidth), command.Clip), toRGBA(command.Stroke))
	}
}

// Rotation pivots on the destination center, mirroring how
// rotated text anchors on its position.
func (r Renderer) drawImage(img *image.RGBA, command render.DrawImage) {
	screenMin := r.Camera.WorldToScreen(command.Dest.Min)
	screenMax := r.Camera.WorldToScreen(command.Dest.Max)

	destX, destY := screenMin.X, screenMin.Y
	destW, destH := screenMax.X-screenMin.X, screenMax.Y-screenMin.Y
	if destW <= 0 || destH <= 0 {
		return
	}

	tex := r.Textures.Lookup(command.Texture)
	if tex == nil {
		if command.Rotation == 0 {
			rect := image.Rect(int(destX), int(destY), int(screenMax.X), int(screenMax.Y))
			fillRect(img, rect.Intersect(r.clipBounds(img, rect, command.Clip)), missingTextureColor)
			return
		}

		// Rotated placeholder: cover the same rotated quad the image would
		// have occupied, so the missing-texture indicator stays truthful
		// about the area it stands in for.
		anchor := math2d.Vec2{X: destX + destW/2, Y: destY + destH/2}
		offsetX, offsetY := -destW/2, -destH/2
		corners := rotatedRectCorners(anchor, offsetX, offsetY, destW, destH, command.Rotation)
		bounds := r.clipBounds(img, rotatedRectBounds(anchor, offsetX, offsetY, destW, destH, command.Rotation), command.Clip)
		fillPolygon(img, corners[:], bounds, missingTextureColor)
		return
	}

	srcRect := tex.Bounds()
	if command.Src != (math2d.Rect{}) {
		srcRect = image.Rect(int(math.Round(command.Src.Min.X)), int(math.Round(command.Src.Min.Y)),
			int(math.Round(command.Src.Max.X)), int(math.Round(command.Src.Max.Y))).Intersect(tex.Bounds())
		if srcRect.Empty() {
			return
		}
	}

	tint := tintFrom(command.Tint)

	if command.Rotation == 0 {
		bounds := r.clipBounds(img, image.Rect(
			int(math.Floor(destX)), int(math.Floor(destY)),
			int(math.Ceil(destX+destW)), int(math.Ceil(destY+destH))), command.Clip)
		blitScaled(img, tex, srcRect, destX, destY, destW, destH, bounds, tint)
		return
	}

	anchor := math2d.Vec2{X: destX + destW/2, Y: destY + destH/2}
	offsetX, offsetY := -destW/2, -destH/2
	bounds := r.clipBounds(img, rotatedRectBounds(anchor, offsetX, offsetY, destW, destH, command.Rotation), command.Clip)
	blitScaledRotated(img, tex, srcRect, anchor, offsetX, offsetY, destW, destH, command.Rotation, bounds, tint)
}

// Warnings below fire at most once per process: a bad DrawImageQuad command
// is almost always a persistent authoring mistake (e.g. a bad quad
// generator), not a one-frame fluke, so repeating it every frame would only
// spam the log without adding information.
var (
	imageQuadDegenerateWarnOnce sync.Once
	imageQuadNonConvexWarnOnce  sync.Once
	imageQuadUninvertibleOnce   sync.Once
)

// drawImageQuad maps a texture onto an arbitrary convex quadrilateral using
// inverse homography sampling: for every destination pixel it solves for
// the (u,v) coordinate in the source unit square and nearest-neighbor
// samples the texture there. A plain axis-aligned, unrotated quad degenerates
// to the same per-pixel math as drawImage's affine blit.
func (r Renderer) drawImageQuad(img *image.RGBA, command render.DrawImageQuad) {
	var corners [4]math2d.Vec2
	for i, corner := range command.Corners {
		corners[i] = r.Camera.WorldToScreen(corner)
	}

	// Check convexity before building the homography (matching
	// platform/wasm/tess/image.go's imageQuadProjection): IsConvexQuad
	// rejects any collinear-triple corner set directly, while
	// HomographyFromQuad only catches a degenerate quad incidentally, via a
	// det()==0 fallback on the assembled matrix, after checks that target a
	// specific corner triple. Checking convexity first means a collinear
	// input always logs "not convex" instead of sometimes slipping past
	// HomographyFromQuad's incidental check.
	if !math2d.IsConvexQuad(corners) {
		imageQuadNonConvexWarnOnce.Do(func() {
			slog.Warn("raster: DrawImageQuad corners are not convex; command skipped")
		})
		return
	}

	h, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		imageQuadDegenerateWarnOnce.Do(func() {
			slog.Warn("raster: DrawImageQuad has degenerate corners; command skipped")
		})
		return
	}

	hinv, ok := h.Inverse()
	if !ok {
		imageQuadUninvertibleOnce.Do(func() {
			slog.Warn("raster: DrawImageQuad homography is not invertible; command skipped")
		})
		return
	}

	bounds := r.clipBounds(img, polygonBounds(corners[:]), command.Clip)

	tex := r.Textures.Lookup(command.Texture)
	if tex == nil {
		fillPolygon(img, corners[:], bounds, missingTextureColor)
		return
	}

	srcRect := tex.Bounds()
	if command.Src != (math2d.Rect{}) {
		srcRect = image.Rect(int(math.Round(command.Src.Min.X)), int(math.Round(command.Src.Min.Y)),
			int(math.Round(command.Src.Max.X)), int(math.Round(command.Src.Max.Y))).Intersect(tex.Bounds())
		if srcRect.Empty() {
			return
		}
	}

	tint := tintFrom(command.Tint)
	srcWidth := float64(srcRect.Dx())
	srcHeight := float64(srcRect.Dy())

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			uv := hinv.Apply(math2d.Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			if uv.X < 0 || uv.X > 1 || uv.Y < 0 || uv.Y > 1 {
				continue
			}

			srcX := srcRect.Min.X + int(uv.X*srcWidth)
			if srcX >= srcRect.Max.X {
				srcX = srcRect.Max.X - 1
			}
			srcY := srcRect.Min.Y + int(uv.Y*srcHeight)
			if srcY >= srcRect.Max.Y {
				srcY = srcRect.Max.Y - 1
			}

			c := tex.RGBAAt(srcX, srcY)
			c = tint.apply(c)
			if c.A == 0 {
				continue
			}
			img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
		}
	}
}

func fillPolygon(img *image.RGBA, points []math2d.Vec2, bounds image.Rectangle, c color.RGBA) {
	if len(points) < 3 {
		return
	}

	minY := points[0].Y
	maxY := points[0].Y
	for _, point := range points[1:] {
		minY = min(minY, point.Y)
		maxY = max(maxY, point.Y)
	}

	bounds = bounds.Intersect(image.Rect(
		bounds.Min.X,
		int(math.Floor(minY)),
		bounds.Max.X,
		int(math.Ceil(maxY))+1,
	))

	intersections := make([]float64, 0, len(points))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		scanY := float64(y) + 0.5
		intersections = intersections[:0]

		for i := range points {
			from := points[i]
			to := points[(i+1)%len(points)]
			if from.Y == to.Y {
				continue
			}

			edgeMinY := min(from.Y, to.Y)
			edgeMaxY := max(from.Y, to.Y)
			if scanY < edgeMinY || scanY >= edgeMaxY {
				continue
			}

			t := (scanY - from.Y) / (to.Y - from.Y)
			intersections = append(intersections, from.X+t*(to.X-from.X))
		}

		sort.Float64s(intersections)
		for i := 0; i+1 < len(intersections); i += 2 {
			startX := int(math.Ceil(intersections[i] - 0.5))
			endX := int(math.Ceil(intersections[i+1] - 0.5))
			if startX < bounds.Min.X {
				startX = bounds.Min.X
			}
			if endX > bounds.Max.X {
				endX = bounds.Max.X
			}
			for x := startX; x < endX; x++ {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

// blitScaled nearest-neighbor scales src into dst at the given destination
// rect, alpha-blending each opaque source pixel over the destination.
func blitScaled(
	dst *image.RGBA,
	src *image.RGBA,
	srcRect image.Rectangle,
	destX,
	destY,
	destWidth,
	destHeight float64,
	bounds image.Rectangle,
	tint tintFactors,
) {
	if destWidth <= 0 || destHeight <= 0 {
		return
	}

	bounds = bounds.Intersect(image.Rect(
		int(math.Floor(destX)),
		int(math.Floor(destY)),
		int(math.Ceil(destX+destWidth)),
		int(math.Ceil(destY+destHeight)),
	))

	scaleX := float64(srcRect.Dx()) / destWidth
	scaleY := float64(srcRect.Dy()) / destHeight

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		srcY := srcRect.Min.Y + int((float64(y)+0.5-destY)*scaleY)
		if srcY < srcRect.Min.Y || srcY >= srcRect.Max.Y {
			continue
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			srcX := srcRect.Min.X + int((float64(x)+0.5-destX)*scaleX)
			if srcX < srcRect.Min.X || srcX >= srcRect.Max.X {
				continue
			}

			c := src.RGBAAt(srcX, srcY)
			c = tint.apply(c)
			if c.A == 0 {
				continue
			}
			dst.SetRGBA(x, y, blendRGBA(dst.RGBAAt(x, y), c))
		}
	}
}

func blitScaledRotated(
	dst *image.RGBA,
	src *image.RGBA,
	srcRect image.Rectangle,
	anchor math2d.Vec2,
	offsetX, offsetY, destWidth, destHeight, angle float64,
	bounds image.Rectangle,
	tint tintFactors,
) {
	if destWidth <= 0 || destHeight <= 0 {
		return
	}

	cosAngle := math.Cos(angle)
	sinAngle := math.Sin(angle)
	scaleX := float64(srcRect.Dx()) / destWidth
	scaleY := float64(srcRect.Dy()) / destHeight

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - anchor.X
			dy := float64(y) + 0.5 - anchor.Y

			localX := dx*cosAngle + dy*sinAngle - offsetX
			localY := -dx*sinAngle + dy*cosAngle - offsetY
			if localX < 0 || localX >= destWidth || localY < 0 || localY >= destHeight {
				continue
			}

			srcX := srcRect.Min.X + int(localX*scaleX)
			srcY := srcRect.Min.Y + int(localY*scaleY)
			if srcX < srcRect.Min.X || srcX >= srcRect.Max.X || srcY < srcRect.Min.Y || srcY >= srcRect.Max.Y {
				continue
			}

			c := src.RGBAAt(srcX, srcY)
			c = tint.apply(c)
			if c.A == 0 {
				continue
			}
			dst.SetRGBA(x, y, blendRGBA(dst.RGBAAt(x, y), c))
		}
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Over)
}

func fillCircle(img *image.RGBA, center math2d.Vec2, radius float64, bounds image.Rectangle, c color.RGBA) {
	radiusSq := radius * radius
	bounds = bounds.Intersect(circleBounds(center, radius))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y

			if dx*dx+dy*dy <= radiusSq {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func strokeRect(img *image.RGBA, rect image.Rectangle, width int, bounds image.Rectangle, c color.RGBA) {
	if width <= 0 || rect.Empty() {
		return
	}

	// A stroke wide enough to cover the interior is a filled rect. Falling
	// back also keeps the bands below from overlapping each other.
	if width*2 >= rect.Dx() || width*2 >= rect.Dy() {
		fillRect(img, rect.Intersect(bounds), c)
		return
	}

	// The four bands must not overlap: fillRect composites with "over", so
	// any shared pixel (the corners, in the naive full-height side bands)
	// would be blended twice and come out darker with a translucent stroke.

	// top
	fillRect(img, image.Rect(
		rect.Min.X,
		rect.Min.Y,
		rect.Max.X,
		rect.Min.Y+width).Intersect(bounds), c)

	// bottom
	fillRect(img, image.Rect(
		rect.Min.X,
		rect.Max.Y-width,
		rect.Max.X,
		rect.Max.Y).Intersect(bounds), c)

	// left (between the top and bottom bands)
	fillRect(img, image.Rect(
		rect.Min.X,
		rect.Min.Y+width,
		rect.Min.X+width,
		rect.Max.Y-width).Intersect(bounds), c)

	// right (between the top and bottom bands)
	fillRect(img, image.Rect(
		rect.Max.X-width,
		rect.Min.Y+width,
		rect.Max.X,
		rect.Max.Y-width).Intersect(bounds), c)
}

func strokeCircle(img *image.RGBA, center math2d.Vec2, radius float64, width float64, bounds image.Rectangle, c color.RGBA) {
	if radius <= 0 || width <= 0 {
		return
	}

	outerRadius := radius
	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	outerSq := outerRadius * outerRadius
	innerSq := innerRadius * innerRadius

	bounds = bounds.Intersect(circleBounds(center, outerRadius))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y
			distSq := dx*dx + dy*dy

			if distSq >= innerSq && distSq <= outerSq {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func strokeArc(img *image.RGBA, center math2d.Vec2, radius, width, startAngle, endAngle float64, bounds image.Rectangle, c color.RGBA) {
	if radius <= 0 || width <= 0 {
		return
	}

	outerRadius := radius
	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	outerSq := outerRadius * outerRadius
	innerSq := innerRadius * innerRadius

	bounds = bounds.Intersect(circleBounds(center, outerRadius))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y

			distSq := dx*dx + dy*dy
			if distSq < innerSq || distSq > outerSq {
				continue
			}

			angle := math.Atan2(dy, dx)
			if angle < 0 {
				angle += math.Pi * 2
			}

			if angleBetween(angle, startAngle, endAngle) {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func fillRoundedRect(img *image.RGBA, rect image.Rectangle, radius float64, bounds image.Rectangle, c color.RGBA) {
	if rect.Empty() {
		return
	}

	radius = clampRoundedRectRadius(rect, radius)
	if radius <= 0 {
		fillRect(img, rect.Intersect(bounds), c)
		return
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if pointInRoundedRect(float64(x)+0.5, float64(y)+0.5, rect, radius) {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func strokeRoundedRect(img *image.RGBA, rect image.Rectangle, radius, width float64, bounds image.Rectangle, c color.RGBA) {
	if rect.Empty() || width <= 0 {
		return
	}

	radius = clampRoundedRectRadius(rect, radius)
	if radius <= 0 {
		strokeRect(img, rect, int(math.Ceil(width)), bounds, c)
		return
	}

	inner := rect.Inset(int(math.Ceil(width)))
	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			if !pointInRoundedRect(px, py, rect, radius) {
				continue
			}
			if !inner.Empty() && pointInRoundedRect(px, py, inner, innerRadius) {
				continue
			}
			img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
		}
	}
}

func pointInRoundedRect(x, y float64, rect image.Rectangle, radius float64) bool {
	if x < float64(rect.Min.X) || x >= float64(rect.Max.X) || y < float64(rect.Min.Y) || y >= float64(rect.Max.Y) {
		return false
	}
	if radius <= 0 {
		return true
	}

	left := float64(rect.Min.X) + radius
	right := float64(rect.Max.X) - radius
	top := float64(rect.Min.Y) + radius
	bottom := float64(rect.Max.Y) - radius

	closestX := min(max(x, left), right)
	closestY := min(max(y, top), bottom)
	dx := x - closestX
	dy := y - closestY
	return dx*dx+dy*dy <= radius*radius
}

func clampRoundedRectRadius(rect image.Rectangle, radius float64) float64 {
	if radius <= 0 {
		return 0
	}

	maxRadius := float64(min(rect.Dx(), rect.Dy())) * 0.5
	if radius > maxRadius {
		return maxRadius
	}
	return radius
}

func (r Renderer) clipBounds(img *image.RGBA, bounds image.Rectangle, clip math2d.Rect) image.Rectangle {
	bounds = bounds.Intersect(img.Bounds())
	if isZeroRect(clip) {
		return bounds
	}

	screenMin := r.Camera.WorldToScreen(clip.Min)
	screenMax := r.Camera.WorldToScreen(clip.Max)
	clipRect := image.Rect(
		int(math.Floor(min(screenMin.X, screenMax.X))),
		int(math.Floor(min(screenMin.Y, screenMax.Y))),
		int(math.Ceil(max(screenMin.X, screenMax.X))),
		int(math.Ceil(max(screenMin.Y, screenMax.Y))),
	)
	return bounds.Intersect(clipRect)
}

func circleBounds(center math2d.Vec2, radius float64) image.Rectangle {
	return image.Rect(
		int(math.Floor(center.X-radius)),
		int(math.Floor(center.Y-radius)),
		int(math.Ceil(center.X+radius))+1,
		int(math.Ceil(center.Y+radius))+1,
	)
}

func lineBounds(from, to math2d.Vec2, width float64) image.Rectangle {
	radius := width * 0.5
	return image.Rect(
		int(math.Floor(min(from.X, to.X)-radius)),
		int(math.Floor(min(from.Y, to.Y)-radius)),
		int(math.Ceil(max(from.X, to.X)+radius))+1,
		int(math.Ceil(max(from.Y, to.Y)+radius))+1,
	)
}

func polygonBounds(points []math2d.Vec2) image.Rectangle {
	if len(points) == 0 {
		return image.Rectangle{}
	}

	minX := points[0].X
	maxX := points[0].X
	minY := points[0].Y
	maxY := points[0].Y
	for _, point := range points[1:] {
		minX = min(minX, point.X)
		maxX = max(maxX, point.X)
		minY = min(minY, point.Y)
		maxY = max(maxY, point.Y)
	}

	return image.Rect(
		int(math.Floor(minX)),
		int(math.Floor(minY)),
		int(math.Ceil(maxX))+1,
		int(math.Ceil(maxY))+1,
	)
}

func rotatedRectBounds(anchor math2d.Vec2, offsetX, offsetY, width, height, angle float64) image.Rectangle {
	if angle == 0 {
		return image.Rect(
			int(math.Floor(anchor.X+offsetX)),
			int(math.Floor(anchor.Y+offsetY)),
			int(math.Ceil(anchor.X+offsetX+width)),
			int(math.Ceil(anchor.Y+offsetY+height)),
		)
	}

	corners := rotatedRectCorners(anchor, offsetX, offsetY, width, height, angle)

	minX := math.Inf(1)
	maxX := math.Inf(-1)
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, corner := range corners {
		minX = min(minX, corner.X)
		maxX = max(maxX, corner.X)
		minY = min(minY, corner.Y)
		maxY = max(maxY, corner.Y)
	}

	return image.Rect(
		int(math.Floor(minX)),
		int(math.Floor(minY)),
		int(math.Ceil(maxX)),
		int(math.Ceil(maxY)),
	)
}

// rotatedRectCorners returns the four corners, in screen space, of a
// destination rect anchored at anchor with the given local-space offset and
// size, rotated by angle around anchor. It mirrors the per-pixel transform
// blitScaledRotated inverts, so the placeholder polygon and the eventual
// image blit always agree on the area they cover.
func rotatedRectCorners(anchor math2d.Vec2, offsetX, offsetY, width, height, angle float64) [4]math2d.Vec2 {
	cosAngle := math.Cos(angle)
	sinAngle := math.Sin(angle)
	local := [4]math2d.Vec2{
		{X: offsetX, Y: offsetY},
		{X: offsetX + width, Y: offsetY},
		{X: offsetX + width, Y: offsetY + height},
		{X: offsetX, Y: offsetY + height},
	}

	var corners [4]math2d.Vec2
	for i, corner := range local {
		corners[i] = math2d.Vec2{
			X: anchor.X + corner.X*cosAngle - corner.Y*sinAngle,
			Y: anchor.Y + corner.X*sinAngle + corner.Y*cosAngle,
		}
	}
	return corners
}

func isZeroRect(rect math2d.Rect) bool {
	return rect.Min == (math2d.Vec2{}) && rect.Max == (math2d.Vec2{})
}

func angleBetween(angle float64, start, end float64) bool {
	angle = normalizeAngle(angle)
	start = normalizeAngle(start)
	end = normalizeAngle(end)

	if start <= end {
		return angle >= start && angle <= end
	}

	return angle >= start || angle <= end
}

func normalizeAngle(angle float64) float64 {
	const tau = math.Pi * 2

	angle = math.Mod(angle, tau)
	if angle < 0 {
		angle += tau
	}

	return angle
}

// toRGBA converts a normalized render.Color into the alpha-premultiplied
// color.RGBA that image.RGBA's pixel format requires.
func toRGBA(c render.Color) color.RGBA {
	return color.RGBA{
		R: toByte(c.R * c.A),
		G: toByte(c.G * c.A),
		B: toByte(c.B * c.A),
		A: toByte(c.A),
	}
}

// blendRGBA composites premultiplied src over premultiplied dst
// (the Porter-Duff "over" operator).
func blendRGBA(dst, src color.RGBA) color.RGBA {
	if src.A == 255 {
		return src
	}
	if src.A == 0 {
		return dst
	}

	inv := uint32(255 - src.A)
	return color.RGBA{
		R: src.R + uint8(uint32(dst.R)*inv/255),
		G: src.G + uint8(uint32(dst.G)*inv/255),
		B: src.B + uint8(uint32(dst.B)*inv/255),
		A: src.A + uint8(uint32(dst.A)*inv/255),
	}
}

func tintFrom(c render.Color) tintFactors {
	if c == (render.Color{}) {
		return noTint
	}
	return tintFactors{r: c.R * c.A, g: c.G * c.A, b: c.B * c.A, a: c.A}
}

func (t tintFactors) apply(c color.RGBA) color.RGBA {
	if t == noTint {
		return c
	}
	return color.RGBA{
		R: toByte(float64(c.R) / 255 * t.r),
		G: toByte(float64(c.G) / 255 * t.g),
		B: toByte(float64(c.B) / 255 * t.b),
		A: toByte(float64(c.A) / 255 * t.a),
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

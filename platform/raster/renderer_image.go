package raster

import (
	"image"
	"image/color"
	"log/slog"
	"math"
	"sync"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

var missingTextureColor = color.RGBA{R: 255, G: 0, B: 255, A: 255}

type resolvedImageDraw struct {
	texture  *image.RGBA
	source   image.Rectangle
	dest     math2d.Rect
	rotation float64
	flip     render.SpriteFlip
	clip     math2d.Rect
	tint     tintFactors
	blend    particle.Blend
}

// Rotation pivots on the destination center, mirroring how
// rotated text anchors on its position.
func (r *Renderer) drawImage(img *image.RGBA, command render.DrawImage) {
	if !command.Transform.IsIdentity() {
		r.drawTransformedImage(img, command)
		return
	}

	tex := r.Textures.Lookup(command.Texture)
	var srcRect image.Rectangle
	if tex != nil {
		srcRect = tex.Bounds()
		if command.Src != (math2d.Rect{}) {
			srcRect = image.Rect(int(math.Round(command.Src.Min.X)), int(math.Round(command.Src.Min.Y)),
				int(math.Round(command.Src.Max.X)), int(math.Round(command.Src.Max.Y))).Intersect(tex.Bounds())
			if srcRect.Empty() {
				return
			}
		}
	}

	r.drawResolvedImage(img, resolvedImageDraw{
		texture:  tex,
		source:   srcRect,
		dest:     command.Dest,
		rotation: command.Rotation,
		flip:     command.Flip,
		clip:     command.Clip,
		tint:     tintFrom(command.Tint),
		blend:    particle.BlendAlpha,
	})
}

// drawTransformedImage renders a DrawImage carrying a non-identity affine
// transform by reusing the DrawImageQuad inverse-sampling path: the
// destination rect's world-space corners (rotated first, if Rotation is set)
// become a quad, and drawImageQuad maps the texture onto the transformed quad.
// This matches the Canvas2D and WebGPU backends instead of drawing a
// placeholder. A missing texture falls out as a magenta transformed quad.
func (r *Renderer) drawTransformedImage(img *image.RGBA, command render.DrawImage) {
	dest := command.Dest
	corners := [4]math2d.Vec2{
		{X: dest.Min.X, Y: dest.Min.Y},
		{X: dest.Max.X, Y: dest.Min.Y},
		{X: dest.Max.X, Y: dest.Max.Y},
		{X: dest.Min.X, Y: dest.Max.Y},
	}
	if command.Rotation != 0 {
		center := math2d.Vec2{X: (dest.Min.X + dest.Max.X) / 2, Y: (dest.Min.Y + dest.Max.Y) / 2}
		sin, cos := math.Sincos(command.Rotation)
		for i, p := range corners {
			dx := p.X - center.X
			dy := p.Y - center.Y
			corners[i] = math2d.Vec2{X: center.X + dx*cos - dy*sin, Y: center.Y + dx*sin + dy*cos}
		}
	}
	corners = render.FlipQuadCorners(corners, command.Flip)
	r.drawImageQuad(img, render.DrawImageQuad{
		BaseCommand: command.BaseCommand,
		Texture:     command.Texture,
		Corners:     corners,
		Src:         command.Src,
		Tint:        command.Tint,
	})
}

func (r *Renderer) drawResolvedImage(img *image.RGBA, command resolvedImageDraw) {
	screenMin := r.Camera.WorldToScreen(command.dest.Min)
	screenMax := r.Camera.WorldToScreen(command.dest.Max)

	destX, destY := screenMin.X, screenMin.Y
	destW, destH := screenMax.X-screenMin.X, screenMax.Y-screenMin.Y
	if destW <= 0 || destH <= 0 {
		return
	}

	if command.texture == nil {
		if command.rotation == 0 {
			rect := image.Rect(int(destX), int(destY), int(screenMax.X), int(screenMax.Y))
			fillRect(img, rect.Intersect(r.clipBounds(img, rect, command.clip)), missingTextureColor)
			return
		}

		// Rotated placeholder: cover the same rotated quad the image would
		// have occupied, so the missing-texture indicator stays truthful
		// about the area it stands in for.
		anchor := math2d.Vec2{X: destX + destW/2, Y: destY + destH/2}
		offsetX, offsetY := -destW/2, -destH/2
		corners := rotatedRectCorners(anchor, offsetX, offsetY, destW, destH, command.rotation)
		bounds := r.clipBounds(img, rotatedRectBounds(anchor, offsetX, offsetY, destW, destH, command.rotation), command.clip)
		fillPolygon(img, corners[:], bounds, missingTextureColor)
		return
	}

	if command.rotation == 0 {
		bounds := r.clipBounds(img, image.Rect(
			int(math.Floor(destX)), int(math.Floor(destY)),
			int(math.Ceil(destX+destW)), int(math.Ceil(destY+destH))), command.clip)
		blitScaledModeFlip(img, command.texture, command.source, destX, destY, destW, destH, bounds, command.tint, command.blend, command.flip)
		return
	}

	anchor := math2d.Vec2{X: destX + destW/2, Y: destY + destH/2}
	offsetX, offsetY := -destW/2, -destH/2
	bounds := r.clipBounds(img, rotatedRectBounds(anchor, offsetX, offsetY, destW, destH, command.rotation), command.clip)
	blitScaledRotatedModeFlip(
		img,
		command.texture,
		command.source,
		anchor,
		offsetX,
		offsetY,
		destW,
		destH,
		command.rotation,
		bounds,
		command.tint,
		command.blend,
		command.flip,
	)
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
func (r *Renderer) drawImageQuad(img *image.RGBA, command render.DrawImageQuad) {
	var corners [4]math2d.Vec2
	for i, corner := range command.Corners {
		corners[i] = r.Camera.WorldToScreen(command.Transform.Apply(corner))
	}

	// Check convexity before building the homography (matching
	// internal/render/gpu/tess/image.go's imageQuadProjection): IsConvexQuad
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
	blitScaledMode(dst, src, srcRect, destX, destY, destWidth, destHeight, bounds, tint, particle.BlendAlpha)
}

func blitScaledMode(
	dst *image.RGBA,
	src *image.RGBA,
	srcRect image.Rectangle,
	destX,
	destY,
	destWidth,
	destHeight float64,
	bounds image.Rectangle,
	tint tintFactors,
	mode particle.Blend,
) {
	blitScaledModeFlip(dst, src, srcRect, destX, destY, destWidth, destHeight, bounds, tint, mode, render.FlipNone)
}

func blitScaledModeFlip(
	dst *image.RGBA,
	src *image.RGBA,
	srcRect image.Rectangle,
	destX,
	destY,
	destWidth,
	destHeight float64,
	bounds image.Rectangle,
	tint tintFactors,
	mode particle.Blend,
	flip render.SpriteFlip,
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
		sampleY := float64(y) + 0.5 - destY
		if flip&render.FlipY != 0 {
			sampleY = destHeight - sampleY
		}
		srcY := srcRect.Min.Y + int(sampleY*scaleY)
		if srcY < srcRect.Min.Y || srcY >= srcRect.Max.Y {
			continue
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sampleX := float64(x) + 0.5 - destX
			if flip&render.FlipX != 0 {
				sampleX = destWidth - sampleX
			}
			srcX := srcRect.Min.X + int(sampleX*scaleX)
			if srcX < srcRect.Min.X || srcX >= srcRect.Max.X {
				continue
			}

			c := src.RGBAAt(srcX, srcY)
			c = tint.apply(c)
			if c.A == 0 {
				continue
			}
			dst.SetRGBA(x, y, blendPixel(dst.RGBAAt(x, y), c, mode))
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
	blitScaledRotatedMode(
		dst,
		src,
		srcRect,
		anchor,
		offsetX,
		offsetY,
		destWidth,
		destHeight,
		angle,
		bounds,
		tint,
		particle.BlendAlpha,
	)
}

func blitScaledRotatedMode(
	dst *image.RGBA,
	src *image.RGBA,
	srcRect image.Rectangle,
	anchor math2d.Vec2,
	offsetX, offsetY, destWidth, destHeight, angle float64,
	bounds image.Rectangle,
	tint tintFactors,
	mode particle.Blend,
) {
	blitScaledRotatedModeFlip(dst, src, srcRect, anchor, offsetX, offsetY, destWidth, destHeight, angle, bounds, tint, mode, render.FlipNone)
}

func blitScaledRotatedModeFlip(
	dst *image.RGBA,
	src *image.RGBA,
	srcRect image.Rectangle,
	anchor math2d.Vec2,
	offsetX, offsetY, destWidth, destHeight, angle float64,
	bounds image.Rectangle,
	tint tintFactors,
	mode particle.Blend,
	flip render.SpriteFlip,
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
			if flip&render.FlipX != 0 {
				localX = destWidth - localX
			}
			if flip&render.FlipY != 0 {
				localY = destHeight - localY
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
			dst.SetRGBA(x, y, blendPixel(dst.RGBAAt(x, y), c, mode))
		}
	}
}

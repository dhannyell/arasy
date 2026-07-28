package raster

import (
	"image"
	"math"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type faceKey struct {
	id render.FontID
	px int
}

func (r *Renderer) face(id render.FontID, px int) font.Face {
	tf := r.Fonts.Lookup(id)
	if tf == nil {
		return nil
	}
	if r.faceCache == nil {
		r.faceCache = map[faceKey]font.Face{}
	}
	key := faceKey{id: id, px: px}
	if f, ok := r.faceCache[key]; ok {
		return f
	}
	f, err := opentype.NewFace(tf.Font(), &opentype.FaceOptions{
		Size: float64(px), DPI: 72, Hinting: font.HintingNone,
	})
	if err != nil {
		return nil
	} // fall back to built-in font
	r.faceCache[key] = f
	return f
}

// defaultFace returns a cached opentype face for the embedded default font
// (FontID 0) at the given pixel size, or nil when the default font was
// compiled out (arr_no_default_font) or cannot be rasterized — callers then
// fall back to the built-in bitmap font.
func (r *Renderer) defaultFace(px int) font.Face {
	def := typeface.DefaultFace()
	if def == nil {
		return nil
	}
	if r.faceCache == nil {
		r.faceCache = map[faceKey]font.Face{}
	}
	key := faceKey{id: 0, px: px}
	if f, ok := r.faceCache[key]; ok {
		return f
	}
	f, err := opentype.NewFace(def.Font(), &opentype.FaceOptions{
		Size: float64(px), DPI: 72, Hinting: font.HintingNone,
	})
	if err != nil {
		return nil
	}
	r.faceCache[key] = f
	return f
}

func (r *Renderer) drawText(img *image.RGBA, command render.DrawText) {
	if command.Text == "" || command.Color.A <= 0 {
		return
	}

	var destWidth, destHeight, offsetX, offsetY, destX, destY float64
	var sourceSpacing float64

	screenPos := r.Camera.WorldToScreen(command.Position)
	screenSpacing := r.Camera.WorldScalarToScreen(command.LetterSpacing)
	size := command.Size

	face := font.Face(nil)
	if command.Font != 0 {
		px := int(math.Round(r.Camera.WorldScalarToScreen(size)))
		if px <= 0 {
			return
		}
		face = r.face(command.Font, px)
	}

	var glyphs *image.RGBA
	var drawer *font.Drawer
	if face != nil {
		adv := font.MeasureString(face, command.Text)
		met := face.Metrics()
		glyphWidth := adv.Ceil()
		runeCount := preparedTextMetrics(command, command.Size).RuneCount
		if runeCount > 1 {
			glyphWidth += int(math.Ceil(float64(runeCount-1) * screenSpacing))
		}
		glyphHeight := (met.Ascent + met.Descent).Ceil()
		if glyphWidth <= 0 || glyphHeight <= 0 {
			return
		}
		glyphs = image.NewRGBA(image.Rect(0, 0, glyphWidth, glyphHeight))

		drawer = &font.Drawer{
			Dst:  glyphs,
			Src:  image.NewUniform(toRGBA(command.Color)),
			Face: face,
			Dot:  fixed.P(0, met.Ascent.Ceil()),
		}

		destWidth = float64(glyphWidth)
		destHeight = float64(glyphHeight)
		sourceSpacing = screenSpacing

		switch command.Align {
		case render.AlignCenter:
			offsetX = -destWidth / 2
		case render.AlignRight:
			offsetX = -destWidth
		case render.AlignLeft:
			offsetX = 0
		}

		switch command.Baseline {
		case render.BaselineTop:
			offsetY = 0
		case render.BaselineMiddle:
			offsetY = -destHeight / 2
		case render.BaselineBottom:
			offsetY = -destHeight
		case render.BaselineAlphabetic:
			offsetY = -float64(met.Ascent.Ceil())
		default:
			offsetY = -float64(met.Ascent.Ceil())
		}
	} else {
		if size <= 0 {
			// Mirror the GPU tessellator: a non-positive size falls back to
			// the built-in font's native pixel height.
			size = float64(basicfont.Face7x13.Metrics().Height.Ceil())
		}
		prepared := preparedTextMetrics(command, size)
		measured := prepared.Size
		ttlDestWidth := r.Camera.WorldScalarToScreen(measured.X)
		ttlDestHeight := r.Camera.WorldScalarToScreen(measured.Y)

		var textFace font.Face
		var metrics font.Metrics
		var isBitmapFace bool
		var nativeAscent float64
		var glyphHeight int

		if ttlDestWidth > 0 && ttlDestHeight > 0 {
			px := int(math.Round(r.Camera.WorldScalarToScreen(size)))
			if px <= 0 {
				return
			}
			if f := r.defaultFace(px); f != nil {
				textFace = f
				metrics = f.Metrics()

				naturalWidth := font.MeasureString(textFace, command.Text).Ceil()
				glyphWidth := naturalWidth
				naturalDestWidth := r.Camera.WorldScalarToScreen(prepared.NaturalWidth)
				if naturalDestWidth != 0 {
					sourceSpacing = screenSpacing * float64(naturalWidth) / naturalDestWidth
				}
				glyphWidth += int(math.Ceil(float64(max(0, prepared.RuneCount-1)) * sourceSpacing))
				glyphHeight = (metrics.Ascent + metrics.Descent).Ceil()
				if glyphWidth > 0 && glyphHeight > 0 {
					glyphs = image.NewRGBA(image.Rect(0, 0, glyphWidth, glyphHeight))
				}
			}
		}

		if textFace == nil {
			isBitmapFace = true
			textFace = basicfont.Face7x13
			metrics = textFace.Metrics()
			nativeAscent = float64(metrics.Ascent.Ceil())
			nativeWidth := float64(font.MeasureString(textFace, command.Text).Ceil())
			if nativeWidth <= 0 {
				return
			}
			if ttlDestWidth <= 0 || ttlDestHeight <= 0 {
				return
			}
			naturalDestWidth := r.Camera.WorldScalarToScreen(prepared.NaturalWidth)
			if naturalDestWidth != 0 {
				sourceSpacing = screenSpacing * nativeWidth / naturalDestWidth
			}
			bufferWidth := nativeWidth + float64(max(0, prepared.RuneCount-1))*sourceSpacing
			if bufferWidth <= 0 {
				return
			}
			glyphs = image.NewRGBA(image.Rect(0, 0, int(math.Ceil(bufferWidth)), metrics.Height.Ceil()))
		}

		destWidth = ttlDestWidth
		destHeight = ttlDestHeight

		if glyphs == nil {
			return
		}

		drawer = &font.Drawer{
			Dst:  glyphs,
			Src:  image.NewUniform(toRGBA(command.Color)),
			Face: textFace,
			Dot:  fixed.Point26_6{X: 0, Y: metrics.Ascent},
		}

		switch command.Align {
		case render.AlignCenter:
			offsetX = -destWidth / 2
		case render.AlignRight:
			offsetX = -destWidth
		case render.AlignLeft:
			offsetX = 0
		}

		switch command.Baseline {
		case render.BaselineTop:
			offsetY = 0
		case render.BaselineMiddle:
			offsetY = -destHeight / 2
		case render.BaselineBottom:
			offsetY = -destHeight
		default: // BaselineAlphabetic
			// The glyph run is stretched into destHeight, so the baseline
			// offset must scale with it — the same convention the GPU
			// tessellator's textTopY uses.
			if isBitmapFace {
				offsetY = -(nativeAscent / float64(metrics.Height.Ceil())) * destHeight
			} else {
				offsetY = -(float64(metrics.Ascent) / 64 / float64(glyphHeight)) * destHeight
			}
		}

	}

	destX = screenPos.X + offsetX
	destY = screenPos.Y + offsetY
	drawSpacedString(drawer, command.Text, sourceSpacing)

	bounds := r.clipBounds(img, rotatedRectBounds(screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation), command.Clip)

	if command.Rotation == 0 && command.Transform.IsIdentity() {
		blitScaled(img, glyphs, glyphs.Bounds(), destX, destY, destWidth, destHeight, bounds, noTint)
		return
	}

	if !command.Transform.IsIdentity() {
		transformClip := r.clipBounds(img, img.Bounds(), command.Clip)
		blitTextTransformed(img, glyphs, screenPos, offsetX, offsetY, destWidth, destHeight, command, r.Camera, transformClip)
		return
	}
	blitScaledRotated(img, glyphs, glyphs.Bounds(), screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation, bounds, noTint)
}

func preparedTextMetrics(command render.DrawText, size float64) render.TextMetrics {
	if command.Metrics.RuneCount > 0 &&
		command.Metrics.Size.X > 0 &&
		command.Metrics.Size.Y > 0 &&
		size == command.Size {
		return command.Metrics
	}
	return render.MeasureTextMetrics(command.Text, size, command.LetterSpacing)
}

func drawSpacedString(drawer *font.Drawer, text string, spacing float64) {
	if spacing == 0 {
		drawer.DrawString(text)
		return
	}
	runes := []rune(text)
	for index, value := range runes {
		drawer.DrawString(string(value))
		if index+1 < len(runes) {
			drawer.Dot.X += fixed.Int26_6(math.Round(spacing * 64))
		}
	}
}

func blitTextTransformed(
	dst *image.RGBA,
	src *image.RGBA,
	anchor math2d.Vec2,
	offsetX, offsetY, width, height float64,
	command render.DrawText,
	camera render.Camera,
	clip image.Rectangle,
) {
	mapPoint := func(point math2d.Vec2) math2d.Vec2 {
		if command.Rotation != 0 {
			sin, cos := math.Sincos(command.Rotation)
			dx, dy := point.X-anchor.X, point.Y-anchor.Y
			point = math2d.Vec2{X: anchor.X + dx*cos - dy*sin, Y: anchor.Y + dx*sin + dy*cos}
		}
		return camera.WorldToScreen(command.Transform.Apply(camera.ScreenToWorld(point)))
	}
	p00 := mapPoint(anchor.Add(math2d.Vec2{X: offsetX, Y: offsetY}))
	p10 := mapPoint(anchor.Add(math2d.Vec2{X: offsetX + width, Y: offsetY}))
	p01 := mapPoint(anchor.Add(math2d.Vec2{X: offsetX, Y: offsetY + height}))
	p11 := p10.Add(p01).Sub(p00)
	srcW, srcH := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	if srcW <= 0 || srcH <= 0 {
		return
	}
	toScreen := math2d.Affine{
		A: (p10.X - p00.X) / srcW, B: (p10.Y - p00.Y) / srcW,
		C: (p01.X - p00.X) / srcH, D: (p01.Y - p00.Y) / srcH,
		E: p00.X, F: p00.Y,
	}
	toSource, ok := toScreen.Inverse()
	if !ok {
		return
	}
	bounds := polygonBounds([]math2d.Vec2{p00, p10, p11, p01}).Intersect(clip)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			point := toSource.Apply(math2d.Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			sx, sy := int(math.Floor(point.X)), int(math.Floor(point.Y))
			if sx < 0 || sy < 0 || sx >= src.Bounds().Dx() || sy >= src.Bounds().Dy() {
				continue
			}
			color := src.RGBAAt(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy)
			if color.A != 0 {
				dst.SetRGBA(x, y, blendRGBA(dst.RGBAAt(x, y), color))
			}
		}
	}
}

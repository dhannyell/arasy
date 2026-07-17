package raster

import (
	"image"
	"math"

	"github.com/dhannyell/frameforge/render"
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

// textFace is the built-in bitmap font used to render DrawText commands.
// Its native pixel height anchors the scale factor derived from
// DrawText.Size, so text grows/shrinks with the camera like other shapes.
var textFace = basicfont.Face7x13

func (r *Renderer) drawText(img *image.RGBA, command render.DrawText) {
	if command.Text == "" || command.Color.A <= 0 {
		return
	}

	var destWidth, destHeight, offsetX, offsetY, destX, destY float64

	screenPos := r.Camera.WorldToScreen(command.Position)
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
		metrics := textFace.Metrics()
		nativeHeight := float64(metrics.Height.Ceil())
		nativeAscent := float64(metrics.Ascent.Ceil())
		nativeWidth := float64(font.MeasureString(textFace, command.Text).Ceil())
		if nativeWidth <= 0 {
			return
		}
		if size <= 0 {
			size = nativeHeight
		}

		measured := render.MeasureText(command.Text, size)
		destWidth = r.Camera.WorldScalarToScreen(measured.X)
		destHeight = r.Camera.WorldScalarToScreen(measured.Y)
		if destWidth <= 0 || destHeight <= 0 {
			return
		}

		glyphs = image.NewRGBA(image.Rect(0, 0, int(math.Ceil(nativeWidth)), int(math.Ceil(nativeHeight))))
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
			offsetY = -(nativeAscent / nativeHeight) * destHeight
		}

	}

	destX = screenPos.X + offsetX
	destY = screenPos.Y + offsetY
	drawer.DrawString(command.Text)

	bounds := r.clipBounds(img, rotatedRectBounds(screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation), command.Clip)

	if command.Rotation == 0 {
		blitScaled(img, glyphs, glyphs.Bounds(), destX, destY, destWidth, destHeight, bounds, noTint)
		return
	}

	blitScaledRotated(img, glyphs, glyphs.Bounds(), screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation, bounds, noTint)
}

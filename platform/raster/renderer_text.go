package raster

import (
	"image"
	"log/slog"
	"sync"

	"math"

	"github.com/dhannyell/arasy/asset/typeface"
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

var textAffineWarnOnce sync.Once

func (r *Renderer) drawText(img *image.RGBA, command render.DrawText) {
	if !command.Transform.IsIdentity() {
		textAffineWarnOnce.Do(func() {
			slog.Warn("raster: DrawText with non-identity Transform is not supported; drawing without transform")
		})
	}

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
		if size <= 0 {
			// Mirror the GPU tessellator: a non-positive size falls back to
			// the built-in font's native pixel height.
			size = float64(basicfont.Face7x13.Metrics().Height.Ceil())
		}
		measured := render.MeasureText(command.Text, size)
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

				glyphWidth := font.MeasureString(textFace, command.Text).Ceil()
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
			glyphs = image.NewRGBA(image.Rect(0, 0, int(math.Ceil(nativeWidth)), metrics.Height.Ceil()))
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
	drawer.DrawString(command.Text)

	bounds := r.clipBounds(img, rotatedRectBounds(screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation), command.Clip)

	if command.Rotation == 0 {
		blitScaled(img, glyphs, glyphs.Bounds(), destX, destY, destWidth, destHeight, bounds, noTint)
		return
	}

	blitScaledRotated(img, glyphs, glyphs.Bounds(), screenPos, offsetX, offsetY, destWidth, destHeight, command.Rotation, bounds, noTint)
}

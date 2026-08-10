package raster

import (
	"image"
	"image/color"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

type postPixel struct {
	r, g, b, a float64
}

func applyPostChain(img *image.RGBA, chain render.PostChain, camera render.Camera) {
	if len(chain.Effects) == 0 {
		return
	}

	current := clonePixels(img)
	for _, effect := range chain.Effects {
		current = applyPostEffect(current, effect, camera, chain.Time)
	}
	copyPixels(img, current)
}

func applyPostEffect(img *image.RGBA, effect render.PostEffect, camera render.Camera, time float64) *image.RGBA {
	switch effect := effect.(type) {
	case render.Vignette:
		return applyVignette(img, effect)
	case render.ColorGrade:
		return applyColorGrade(img, effect)
	case render.Pixelate:
		return applyPixelate(img, effect)
	case render.ChromaticAberration:
		return applyChromaticAberration(img, effect)
	case render.Bloom:
		return applyBloom(img, effect)
	case render.Shockwave:
		return applyShockwave(img, effect, camera)
	case render.LensRain:
		return applyLensRain(img, effect, time)
	default:
		return img
	}
}

func applyVignette(src *image.RGBA, effect render.Vignette) *image.RGBA {
	if effect.Strength == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	width := float64(src.Bounds().Dx())
	height := float64(src.Bounds().Dy())
	softness := clamp01(effect.Softness)
	strength := clamp01(effect.Strength)
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		py := ((float64(y-src.Bounds().Min.Y) + 0.5) / height) * 2
		py--
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			px := ((float64(x-src.Bounds().Min.X) + 0.5) / width) * 2
			px--
			distance := math.Hypot(px, py) / math.Sqrt2
			t := strength * smoothstep(1-softness, 1, distance)
			pixel := pixelAt(src, x, y)
			pixel.r += (effect.Color.R - pixel.r) * t
			pixel.g += (effect.Color.G - pixel.g) * t
			pixel.b += (effect.Color.B - pixel.b) * t
			setPixel(dst, x, y, pixel)
		}
	}
	return dst
}

func applyColorGrade(src *image.RGBA, effect render.ColorGrade) *image.RGBA {
	if effect == (render.ColorGrade{}) {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	contrast := 1 + effect.Contrast
	saturation := 1 + effect.Saturation
	tint := effect.Tint
	if tint == (render.Color{}) {
		tint = render.RGBA(1, 1, 1, 1)
	}
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			pixel := pixelAt(src, x, y)
			pixel.r = (pixel.r-0.5)*contrast + 0.5 + effect.Brightness
			pixel.g = (pixel.g-0.5)*contrast + 0.5 + effect.Brightness
			pixel.b = (pixel.b-0.5)*contrast + 0.5 + effect.Brightness

			luma := 0.2126*pixel.r + 0.7152*pixel.g + 0.0722*pixel.b
			pixel.r = luma + (pixel.r-luma)*saturation
			pixel.g = luma + (pixel.g-luma)*saturation
			pixel.b = luma + (pixel.b-luma)*saturation
			pixel.r *= tint.R
			pixel.g *= tint.G
			pixel.b *= tint.B
			setPixel(dst, x, y, pixel)
		}
	}
	return dst
}

func applyPixelate(src *image.RGBA, effect render.Pixelate) *image.RGBA {
	if effect.Size <= 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sourceX := math.Floor(float64(x-bounds.Min.X)/effect.Size)*effect.Size + effect.Size*0.5
			sourceY := math.Floor(float64(y-bounds.Min.Y)/effect.Size)*effect.Size + effect.Size*0.5
			sourceX = math.Max(0, math.Min(float64(bounds.Dx()-1), sourceX))
			sourceY = math.Max(0, math.Min(float64(bounds.Dy()-1), sourceY))
			setPixel(dst, x, y, pixelAt(src, bounds.Min.X+int(sourceX), bounds.Min.Y+int(sourceY)))
		}
	}
	return dst
}

func applyChromaticAberration(src *image.RGBA, effect render.ChromaticAberration) *image.RGBA {
	if effect.Strength == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	width := float64(src.Bounds().Dx())
	height := float64(src.Bounds().Dy())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		py := ((float64(y-src.Bounds().Min.Y) + 0.5) / height) * 2
		py--
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			px := ((float64(x-src.Bounds().Min.X) + 0.5) / width) * 2
			px--
			magnitude := px*px + py*py
			offsetX := px * magnitude * effect.Strength
			offsetY := py * magnitude * effect.Strength
			red := sampleBilinear(src, float64(x)+offsetX, float64(y)+offsetY)
			blue := sampleBilinear(src, float64(x)-offsetX, float64(y)-offsetY)
			pixel := pixelAt(src, x, y)
			pixel.r = red.r
			pixel.b = blue.b
			setPixel(dst, x, y, pixel)
		}
	}
	return dst
}

type postBuffer struct {
	width, height int
	pixels        []postPixel
}

func newPostBuffer(width, height int) *postBuffer {
	return &postBuffer{width: width, height: height, pixels: make([]postPixel, width*height)}
}

func (b *postBuffer) set(x, y int, pixel postPixel) {
	b.pixels[y*b.width+x] = pixel
}

func (b *postBuffer) at(x, y int) postPixel {
	x = max(0, min(b.width-1, x))
	y = max(0, min(b.height-1, y))
	return b.pixels[y*b.width+x]
}

func (b *postBuffer) sample(x, y float64) postPixel {
	x = math.Max(0, math.Min(float64(b.width-1), x))
	y = math.Max(0, math.Min(float64(b.height-1), y))
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, b.width-1), min(y0+1, b.height-1)
	tx, ty := x-float64(x0), y-float64(y0)
	p0 := b.at(x0, y0)
	p1 := b.at(x1, y0)
	p2 := b.at(x0, y1)
	p3 := b.at(x1, y1)
	return postPixel{
		r: bilerp(p0.r, p1.r, p2.r, p3.r, tx, ty),
		g: bilerp(p0.g, p1.g, p2.g, p3.g, tx, ty),
		b: bilerp(p0.b, p1.b, p2.b, p3.b, tx, ty),
		a: bilerp(p0.a, p1.a, p2.a, p3.a, tx, ty),
	}
}

func applyBloom(src *image.RGBA, effect render.Bloom) *image.RGBA {
	if effect.Intensity == 0 {
		return src
	}

	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	passes = max(1, min(5, passes))

	bright := newPostBuffer(src.Bounds().Dx(), src.Bounds().Dy())
	for y := 0; y < bright.height; y++ {
		for x := 0; x < bright.width; x++ {
			pixel := pixelAt(src, src.Bounds().Min.X+x, src.Bounds().Min.Y+y)
			bright.set(x, y, postPixel{
				r: math.Max(pixel.r-effect.Threshold, 0),
				g: math.Max(pixel.g-effect.Threshold, 0),
				b: math.Max(pixel.b-effect.Threshold, 0),
				a: pixel.a,
			})
		}
	}

	levels := make([]*postBuffer, passes+1)
	levels[0] = bright
	for level := 1; level <= passes; level++ {
		levels[level] = downsampleKawase(levels[level-1])
	}

	bloom := levels[passes]
	for level := passes - 1; level >= 0; level-- {
		bloom = upsampleKawase(bloom, levels[level].width, levels[level].height)
	}

	dst := clonePixels(src)
	for y := 0; y < bloom.height; y++ {
		for x := 0; x < bloom.width; x++ {
			pixel := pixelAt(dst, dst.Bounds().Min.X+x, dst.Bounds().Min.Y+y)
			blurred := bloom.at(x, y)
			pixel.r += blurred.r * effect.Intensity
			pixel.g += blurred.g * effect.Intensity
			pixel.b += blurred.b * effect.Intensity
			setPixel(dst, dst.Bounds().Min.X+x, dst.Bounds().Min.Y+y, pixel)
		}
	}
	return dst
}

func downsampleKawase(src *postBuffer) *postBuffer {
	dst := newPostBuffer(max(1, (src.width+1)/2), max(1, (src.height+1)/2))
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {
			centerX := (float64(x)+0.5)*float64(src.width)/float64(dst.width) - 0.5
			centerY := (float64(y)+0.5)*float64(src.height)/float64(dst.height) - 0.5
			center := src.sample(centerX, centerY)
			sum := postPixel{
				r: center.r * 4,
				g: center.g * 4,
				b: center.b * 4,
				a: center.a * 4,
			}
			for _, offset := range [][2]float64{{-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5}, {0.5, 0.5}} {
				pixel := src.sample(centerX+offset[0], centerY+offset[1])
				sum.r += pixel.r
				sum.g += pixel.g
				sum.b += pixel.b
				sum.a += pixel.a
			}
			dst.set(x, y, postPixel{r: sum.r / 8, g: sum.g / 8, b: sum.b / 8, a: sum.a / 8})
		}
	}
	return dst
}

func upsampleKawase(src *postBuffer, width, height int) *postBuffer {
	dst := newPostBuffer(width, height)
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {
			centerX := (float64(x)+0.5)*float64(src.width)/float64(dst.width) - 0.5
			centerY := (float64(y)+0.5)*float64(src.height)/float64(dst.height) - 0.5
			sum := postPixel{}
			for _, tap := range [][3]float64{
				{-1, 0, 2}, {1, 0, 2}, {0, -1, 2}, {0, 1, 2},
				{-1, -1, 1}, {1, -1, 1}, {-1, 1, 1}, {1, 1, 1},
			} {
				pixel := src.sample(centerX+tap[0], centerY+tap[1])
				sum.r += pixel.r * tap[2]
				sum.g += pixel.g * tap[2]
				sum.b += pixel.b * tap[2]
				sum.a += pixel.a * tap[2]
			}
			dst.set(x, y, postPixel{r: sum.r / 12, g: sum.g / 12, b: sum.b / 12, a: sum.a / 12})
		}
	}
	return dst
}

func applyShockwave(src *image.RGBA, effect render.Shockwave, camera render.Camera) *image.RGBA {
	if effect.Width == 0 || effect.Strength == 0 {
		return src
	}

	center := camera.WorldToScreen(effect.Center)
	radius := math.Abs(camera.WorldScalarToScreen(effect.Radius))
	width := math.Abs(camera.WorldScalarToScreen(effect.Width))
	if width == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			direction := math2d.Vec2{X: float64(x) + 0.5 - center.X, Y: float64(y) + 0.5 - center.Y}
			distance := direction.Len()
			band := math.Max(0, 1-math.Abs(distance-radius)/width)
			pixel := pixelAt(src, x, y)
			if distance > 0 && band > 0 {
				offset := direction.Mul(effect.Strength * band * band / distance)
				pixel = sampleBilinear(src, float64(x)-offset.X, float64(y)-offset.Y)
			}
			setPixel(dst, x, y, pixel)
		}
	}
	return dst
}

func applyLensRain(src *image.RGBA, effect render.LensRain, time float64) *image.RGBA {
	density := clamp01(effect.Density)
	if density == 0 {
		return src
	}

	size := effect.Size
	if size <= 0 {
		size = 48
	}
	speed := effect.Speed
	if speed == 0 {
		speed = 0.15
	}

	dst := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			setPixel(dst, x, y, lensRainPixel(src, x, y, density, size, speed, effect.Refraction, effect.Seed, time))
		}
	}
	return dst
}

func lensRainPixel(src *image.RGBA, x, y int, density, size, speed, refraction float64, seed uint64, time float64) postPixel {
	cellX := math.Floor(float64(x) / size)
	cellY := math.Floor(float64(y) / size)
	hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(seed)})
	h := [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
	if h[0] >= density {
		return pixelAt(src, x, y)
	}

	centerX := (cellX + h[1]) * size
	fall := fract(h[0] - time*speed*(0.6+0.8*h[0]))
	centerY := (cellY + fall) * size
	radius := size * (0.12 + 0.08*h[2])
	delta := math2d.Vec2{X: float64(x) + 0.5 - centerX, Y: float64(y) + 0.5 - centerY}
	distance := delta.Len()
	mask := smoothstep(radius, radius*0.8, distance)
	pixel := pixelAt(src, x, y)
	if mask == 0 {
		return pixel
	}

	if refraction > 0 && distance > 0 {
		offset := delta.Mul((1 - distance/radius) * refraction * size / distance)
		pixel = sampleBilinear(src, float64(x)-offset.X, float64(y)-offset.Y)
	}

	pixel.r += (pixel.r*0.55 + 0.32 - pixel.r) * mask
	pixel.g += (pixel.g*0.55 + 0.32 - pixel.g) * mask
	pixel.b += (pixel.b*0.55 + 0.32 - pixel.b) * mask
	return pixel
}

func fract(value float64) float64 {
	return value - math.Floor(value)
}

func smoothstep(edge0, edge1, value float64) float64 {
	if edge0 == edge1 {
		if value < edge0 {
			return 0
		}
		return 1
	}
	t := clamp01((value - edge0) / (edge1 - edge0))
	return t * t * (3 - 2*t)
}

func clonePixels(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copyPixels(dst, src)
	return dst
}

func copyPixels(dst, src *image.RGBA) {
	if dst.Bounds() == src.Bounds() && dst.Stride == src.Stride {
		copy(dst.Pix, src.Pix)
		return
	}

	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(x, y))
		}
	}
}

func sampleBilinear(img *image.RGBA, x, y float64) postPixel {
	bounds := img.Bounds()
	if bounds.Empty() {
		return postPixel{}
	}

	x = math.Max(float64(bounds.Min.X), math.Min(float64(bounds.Max.X-1), x))
	y = math.Max(float64(bounds.Min.Y), math.Min(float64(bounds.Max.Y-1), y))

	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, bounds.Max.X-1), min(y0+1, bounds.Max.Y-1)
	tx, ty := x-float64(x0), y-float64(y0)

	a := pixelAt(img, x0, y0)
	b := pixelAt(img, x1, y0)
	c := pixelAt(img, x0, y1)
	d := pixelAt(img, x1, y1)
	return postPixel{
		r: bilerp(a.r, b.r, c.r, d.r, tx, ty),
		g: bilerp(a.g, b.g, c.g, d.g, tx, ty),
		b: bilerp(a.b, b.b, c.b, d.b, tx, ty),
		a: bilerp(a.a, b.a, c.a, d.a, tx, ty),
	}
}

func bilerp(a, b, c, d, tx, ty float64) float64 {
	return (a + (b-a)*tx) + ((c+(d-c)*tx)-(a+(b-a)*tx))*ty
}

func pixelAt(img *image.RGBA, x, y int) postPixel {
	c := img.RGBAAt(x, y)
	return postPixel{
		r: float64(c.R) / 255,
		g: float64(c.G) / 255,
		b: float64(c.B) / 255,
		a: float64(c.A) / 255,
	}
}

func setPixel(img *image.RGBA, x, y int, p postPixel) {
	img.SetRGBA(x, y, color.RGBA{
		R: toByte(clamp01(p.r)),
		G: toByte(clamp01(p.g)),
		B: toByte(clamp01(p.b)),
		A: toByte(clamp01(p.a)),
	})
}

func clamp01(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 1
	}
	return value
}

package raster

import (
	"image"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

type postPixel struct {
	r, g, b, a float64
}

// postScratch holds the reusable buffers of the post-effect chain. The
// renderer keeps one across frames so a steady chain allocates nothing; every
// buffer is fully overwritten before it is read.
type postScratch struct {
	scene    *image.RGBA
	pingpong *image.RGBA
	buffers  []*postBuffer
}

// buffer returns the pooled float buffer for slot, reallocating only when the
// requested dimensions change.
func (s *postScratch) buffer(slot, width, height int) *postBuffer {
	for len(s.buffers) <= slot {
		s.buffers = append(s.buffers, nil)
	}
	b := s.buffers[slot]
	if b == nil || b.width != width || b.height != height {
		b = newPostBuffer(width, height)
		s.buffers[slot] = b
	}
	return b
}

func ensureImage(current *image.RGBA, bounds image.Rectangle) *image.RGBA {
	if current != nil && current.Bounds() == bounds {
		return current
	}
	return image.NewRGBA(bounds)
}

func applyPostChain(img *image.RGBA, chain render.PostChain, camera render.Camera, sc *postScratch) {
	if len(chain.Effects) == 0 {
		return
	}
	if sc == nil {
		sc = &postScratch{}
	}

	current := img
	var scratch *image.RGBA
	for _, effect := range chain.Effects {
		if postEffectIdentity(effect, camera) {
			continue
		}
		if scratch == nil {
			sc.pingpong = ensureImage(sc.pingpong, img.Bounds())
			scratch = sc.pingpong
		}
		if !applyPostEffectInto(scratch, current, effect, camera, chain.Time, sc) {
			continue
		}
		current, scratch = scratch, current
	}
	if current != img {
		copyPixels(img, current)
	}
}

func postEffectIdentity(effect render.PostEffect, camera render.Camera) bool {
	switch effect := effect.(type) {
	case render.Vignette:
		return effect.Strength == 0
	case render.ColorGrade:
		return effect == (render.ColorGrade{})
	case render.Pixelate:
		return effect.Size <= 0
	case render.ChromaticAberration:
		return effect.Strength == 0
	case render.Bloom:
		return effect.Intensity == 0
	case render.Shockwave:
		return effect.Width == 0 || effect.Strength == 0 ||
			math.Abs(camera.WorldScalarToScreen(effect.Width)) == 0
	case render.LensRain:
		return clamp01(effect.Density) == 0
	case render.Blur:
		return clamp01(effect.Strength) == 0
	case render.Fade:
		return clamp01(effect.Amount) == 0
	case render.CircleWipe:
		return effect.Progress <= 0
	case render.Dissolve:
		return clamp01(effect.Amount) <= 0
	case render.FilmGrain:
		return effect.Intensity == 0
	case render.Scanlines:
		return clamp01(effect.Intensity) == 0 && clamp01(effect.MaskStrength) == 0
	case render.RadialBlur:
		return clamp01(effect.Strength) == 0
	default:
		return false
	}
}

func applyPostEffectInto(dst, src *image.RGBA, effect render.PostEffect, camera render.Camera, time float64, sc *postScratch) bool {
	switch effect := effect.(type) {
	case render.Vignette:
		return applyVignetteInto(dst, src, effect)
	case render.ColorGrade:
		return applyColorGradeInto(dst, src, effect)
	case render.Pixelate:
		return applyPixelateInto(dst, src, effect)
	case render.ChromaticAberration:
		return applyChromaticAberrationInto(dst, src, effect)
	case render.Bloom:
		return applyBloomInto(dst, src, effect, sc)
	case render.Shockwave:
		return applyShockwaveInto(dst, src, effect, camera)
	case render.LensRain:
		return applyLensRainInto(dst, src, effect, time)
	case render.Blur:
		return applyBlurInto(dst, src, effect, sc)
	case render.Fade:
		return applyFadeInto(dst, src, effect)
	case render.CircleWipe:
		return applyCircleWipeInto(dst, src, effect, camera)
	case render.Dissolve:
		return applyDissolveInto(dst, src, effect)
	case render.FilmGrain:
		return applyFilmGrainInto(dst, src, effect, time)
	case render.Scanlines:
		return applyScanlinesInto(dst, src, effect)
	case render.RadialBlur:
		return applyRadialBlurInto(dst, src, effect, camera)
	default:
		return false
	}
}

func applyVignette(src *image.RGBA, effect render.Vignette) *image.RGBA {
	if effect.Strength == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyVignetteInto(dst, src, effect)
	return dst
}

func applyVignetteInto(dst, src *image.RGBA, effect render.Vignette) bool {
	if effect.Strength == 0 {
		return false
	}

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
	return true
}

func applyColorGrade(src *image.RGBA, effect render.ColorGrade) *image.RGBA {
	if effect == (render.ColorGrade{}) {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyColorGradeInto(dst, src, effect)
	return dst
}

func applyColorGradeInto(dst, src *image.RGBA, effect render.ColorGrade) bool {
	if effect == (render.ColorGrade{}) {
		return false
	}

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
	return true
}

func applyPixelate(src *image.RGBA, effect render.Pixelate) *image.RGBA {
	if effect.Size <= 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyPixelateInto(dst, src, effect)
	return dst
}

func applyPixelateInto(dst, src *image.RGBA, effect render.Pixelate) bool {
	if effect.Size <= 0 {
		return false
	}

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
	return true
}

func applyChromaticAberration(src *image.RGBA, effect render.ChromaticAberration) *image.RGBA {
	if effect.Strength == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyChromaticAberrationInto(dst, src, effect)
	return dst
}

func applyChromaticAberrationInto(dst, src *image.RGBA, effect render.ChromaticAberration) bool {
	if effect.Strength == 0 {
		return false
	}

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
	return true
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
	// Comparison clamps replace math.Max/Min: the coordinates are always
	// finite here, so the results are bit-identical and the calls disappear
	// from the hottest loop of the bloom.
	if x < 0 {
		x = 0
	} else if limit := float64(b.width - 1); x > limit {
		x = limit
	}
	if y < 0 {
		y = 0
	} else if limit := float64(b.height - 1); y > limit {
		y = limit
	}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, b.width-1), min(y0+1, b.height-1)
	tx, ty := x-float64(x0), y-float64(y0)
	// The clamps above bound every index, so the pixels are read directly
	// instead of through the re-clamping at helper.
	row0, row1 := y0*b.width, y1*b.width
	p0 := b.pixels[row0+x0]
	p1 := b.pixels[row0+x1]
	p2 := b.pixels[row1+x0]
	p3 := b.pixels[row1+x1]
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

	dst := image.NewRGBA(src.Bounds())
	applyBloomInto(dst, src, effect, &postScratch{})
	return dst
}

func applyBloomInto(dst, src *image.RGBA, effect render.Bloom, sc *postScratch) bool {
	if effect.Intensity == 0 {
		return false
	}

	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	passes = max(1, min(5, passes))

	bright := sc.buffer(0, src.Bounds().Dx(), src.Bounds().Dy())
	for y := 0; y < bright.height; y++ {
		for x := 0; x < bright.width; x++ {
			pixel := pixelAt(src, src.Bounds().Min.X+x, src.Bounds().Min.Y+y)
			// max(v-t, 0) written as comparisons; a-b of equal finite
			// values is +0, so the result matches math.Max bit for bit.
			r, g, b := pixel.r-effect.Threshold, pixel.g-effect.Threshold, pixel.b-effect.Threshold
			if r < 0 {
				r = 0
			}
			if g < 0 {
				g = 0
			}
			if b < 0 {
				b = 0
			}
			bright.set(x, y, postPixel{r: r, g: g, b: b, a: pixel.a})
		}
	}

	// Slots 0..passes hold the downsample pyramid; slots passes+1..2*passes
	// hold the upsample results, one per pyramid level above the smallest.
	levels := make([]*postBuffer, passes+1)
	levels[0] = bright
	for level := 1; level <= passes; level++ {
		source := levels[level-1]
		levels[level] = sc.buffer(level, max(1, (source.width+1)/2), max(1, (source.height+1)/2))
		downsampleKawase(levels[level], source)
	}

	bloom := levels[passes]
	for level := passes - 1; level >= 0; level-- {
		up := sc.buffer(passes+1+level, levels[level].width, levels[level].height)
		upsampleKawase(up, bloom)
		bloom = up
	}

	copyPixels(dst, src)
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
	return true
}

// kawaseDownTaps and kawaseUpTaps live at package level so the sampling loops
// do not rebuild the literals per pixel.
var kawaseDownTaps = [4][2]float64{{-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5}, {0.5, 0.5}}

var kawaseUpTaps = [8][3]float64{
	{-1, 0, 2}, {1, 0, 2}, {0, -1, 2}, {0, 1, 2},
	{-1, -1, 1}, {1, -1, 1}, {-1, 1, 1}, {1, 1, 1},
}

func downsampleKawase(dst, src *postBuffer) {
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
			for _, offset := range kawaseDownTaps {
				pixel := src.sample(centerX+offset[0], centerY+offset[1])
				sum.r += pixel.r
				sum.g += pixel.g
				sum.b += pixel.b
				sum.a += pixel.a
			}
			dst.set(x, y, postPixel{r: sum.r / 8, g: sum.g / 8, b: sum.b / 8, a: sum.a / 8})
		}
	}
}

func upsampleKawase(dst, src *postBuffer) {
	for y := 0; y < dst.height; y++ {
		for x := 0; x < dst.width; x++ {
			centerX := (float64(x)+0.5)*float64(src.width)/float64(dst.width) - 0.5
			centerY := (float64(y)+0.5)*float64(src.height)/float64(dst.height) - 0.5
			sum := postPixel{}
			for _, tap := range kawaseUpTaps {
				pixel := src.sample(centerX+tap[0], centerY+tap[1])
				sum.r += pixel.r * tap[2]
				sum.g += pixel.g * tap[2]
				sum.b += pixel.b * tap[2]
				sum.a += pixel.a * tap[2]
			}
			dst.set(x, y, postPixel{r: sum.r / 12, g: sum.g / 12, b: sum.b / 12, a: sum.a / 12})
		}
	}
}

func applyBlur(src *image.RGBA, effect render.Blur) *image.RGBA {
	if clamp01(effect.Strength) == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyBlurInto(dst, src, effect, &postScratch{})
	return dst
}

// applyBlurInto reuses the Kawase pyramid from applyBloomInto, but seeds level
// zero with the whole image instead of a bright-pass, so the pyramid blurs
// everything rather than only bloom highlights.
func applyBlurInto(dst, src *image.RGBA, effect render.Blur, sc *postScratch) bool {
	strength := clamp01(effect.Strength)
	if strength == 0 {
		return false
	}

	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	passes = max(1, min(5, passes))

	base := sc.buffer(0, src.Bounds().Dx(), src.Bounds().Dy())
	for y := 0; y < base.height; y++ {
		for x := 0; x < base.width; x++ {
			pixel := pixelAt(src, src.Bounds().Min.X+x, src.Bounds().Min.Y+y)
			base.set(x, y, postPixel{r: pixel.r, g: pixel.g, b: pixel.b, a: pixel.a})
		}
	}

	// Slots mirror applyBloomInto's pyramid layout so the two effects share
	// the same scratch pool without either one growing extra buffers.
	levels := make([]*postBuffer, passes+1)
	levels[0] = base
	for level := 1; level <= passes; level++ {
		source := levels[level-1]
		levels[level] = sc.buffer(level, max(1, (source.width+1)/2), max(1, (source.height+1)/2))
		downsampleKawase(levels[level], source)
	}

	blurred := levels[passes]
	for level := passes - 1; level >= 0; level-- {
		up := sc.buffer(passes+1+level, levels[level].width, levels[level].height)
		upsampleKawase(up, blurred)
		blurred = up
	}

	for y := 0; y < blurred.height; y++ {
		for x := 0; x < blurred.width; x++ {
			sharp := pixelAt(src, src.Bounds().Min.X+x, src.Bounds().Min.Y+y)
			soft := blurred.at(x, y)
			setPixel(dst, dst.Bounds().Min.X+x, dst.Bounds().Min.Y+y, postPixel{
				r: sharp.r + (soft.r-sharp.r)*strength,
				g: sharp.g + (soft.g-sharp.g)*strength,
				b: sharp.b + (soft.b-sharp.b)*strength,
				a: sharp.a + (soft.a-sharp.a)*strength,
			})
		}
	}
	return true
}

func applyFade(src *image.RGBA, effect render.Fade) *image.RGBA {
	if clamp01(effect.Amount) == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyFadeInto(dst, src, effect)
	return dst
}

func applyFadeInto(dst, src *image.RGBA, effect render.Fade) bool {
	amount := clamp01(effect.Amount)
	if amount == 0 {
		return false
	}

	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			pixel := pixelAt(src, x, y)
			pixel.r += (effect.Color.R - pixel.r) * amount
			pixel.g += (effect.Color.G - pixel.g) * amount
			pixel.b += (effect.Color.B - pixel.b) * amount
			setPixel(dst, x, y, pixel)
		}
	}
	return true
}

func applyCircleWipe(src *image.RGBA, effect render.CircleWipe, camera render.Camera) *image.RGBA {
	if effect.Progress <= 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyCircleWipeInto(dst, src, effect, camera)
	return dst
}

// applyCircleWipeInto shrinks the visible circle from the farthest screen
// corner to zero as Progress goes 0..1, feathering the edge with smoothstep.
func applyCircleWipeInto(dst, src *image.RGBA, effect render.CircleWipe, camera render.Camera) bool {
	if effect.Progress <= 0 {
		return false
	}

	center := camera.WorldToScreen(effect.Center)
	bounds := src.Bounds()
	corners := [4]math2d.Vec2{
		{X: float64(bounds.Min.X), Y: float64(bounds.Min.Y)},
		{X: float64(bounds.Max.X), Y: float64(bounds.Min.Y)},
		{X: float64(bounds.Min.X), Y: float64(bounds.Max.Y)},
		{X: float64(bounds.Max.X), Y: float64(bounds.Max.Y)},
	}
	farthest := 0.0
	for _, corner := range corners {
		if d := corner.Sub(center).Len(); d > farthest {
			farthest = d
		}
	}
	progress := clamp01(effect.Progress)
	radius := farthest * (1 - progress)
	softness := clamp01(effect.Softness)
	edge0 := radius * (1 - softness)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			distance := (math2d.Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5}).Sub(center).Len()
			t := smoothstep(edge0, radius, distance)
			pixel := pixelAt(src, x, y)
			pixel.r += (effect.Color.R - pixel.r) * t
			pixel.g += (effect.Color.G - pixel.g) * t
			pixel.b += (effect.Color.B - pixel.b) * t
			setPixel(dst, x, y, pixel)
		}
	}
	return true
}

func applyDissolve(src *image.RGBA, effect render.Dissolve) *image.RGBA {
	if clamp01(effect.Amount) <= 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyDissolveInto(dst, src, effect)
	return dst
}

// applyDissolveInto hashes each screen cell once, up front, instead of once
// per covered pixel: dissolve cells are large relative to pixel count, so the
// per-cell pass is a small fraction of the per-pixel work below it.
func applyDissolveInto(dst, src *image.RGBA, effect render.Dissolve) bool {
	amount := clamp01(effect.Amount)
	if amount <= 0 {
		return false
	}

	cellSize := effect.CellSize
	if cellSize <= 0 {
		cellSize = 8
	}

	bounds := src.Bounds()
	cols := int(math.Ceil(float64(bounds.Dx()) / cellSize))
	rows := int(math.Ceil(float64(bounds.Dy()) / cellSize))
	covered := make([]bool, cols*rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			hash := particle.Hash3([3]uint32{uint32(col), uint32(row), uint32(effect.Seed)})
			covered[row*cols+col] = particle.Float01(hash[0]) < amount
		}
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := int(float64(y-bounds.Min.Y) / cellSize)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			col := int(float64(x-bounds.Min.X) / cellSize)
			if !covered[row*cols+col] {
				setPixel(dst, x, y, pixelAt(src, x, y))
				continue
			}
			setPixel(dst, x, y, postPixel{r: effect.Color.R, g: effect.Color.G, b: effect.Color.B, a: effect.Color.A})
		}
	}
	return true
}

func applyFilmGrain(src *image.RGBA, effect render.FilmGrain, time float64) *image.RGBA {
	if effect.Intensity == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyFilmGrainInto(dst, src, effect, time)
	return dst
}

// applyFilmGrainInto hashes each pixel with the tick-quantized time folded
// into the seed lane, so the grain is a pure function of (x, y, tick) and
// animates once per simulation tick instead of every render call.
func applyFilmGrainInto(dst, src *image.RGBA, effect render.FilmGrain, time float64) bool {
	if effect.Intensity == 0 {
		return false
	}

	tick := uint32(time * 60)
	seed := uint32(effect.Seed) + tick
	size := effect.Size
	if size <= 0 {
		size = 1
	}
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		gy := uint32(float64(y-bounds.Min.Y) / size)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			hash := particle.Hash3([3]uint32{uint32(float64(x-bounds.Min.X) / size), gy, seed})
			// Two hash lanes give a triangular distribution: most grains are
			// weak, a few are strong, like photographic grain.
			noise := particle.Float01(hash[0]) + particle.Float01(hash[1]) - 1
			pixel := pixelAt(src, x, y)
			// Grain peaks in the mid-tones and fades in deep shadow and
			// highlight, where film shows almost none.
			luma := 0.2126*pixel.r + 0.7152*pixel.g + 0.0722*pixel.b
			grain := noise * effect.Intensity * clamp01(luma*(1-luma)*4)
			pixel.r += grain
			pixel.g += grain
			pixel.b += grain
			setPixel(dst, x, y, pixel)
		}
	}
	return true
}

func applyScanlines(src *image.RGBA, effect render.Scanlines) *image.RGBA {
	if clamp01(effect.Intensity) == 0 && clamp01(effect.MaskStrength) == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyScanlinesInto(dst, src, effect)
	return dst
}

func applyScanlinesInto(dst, src *image.RGBA, effect render.Scanlines) bool {
	intensity := clamp01(effect.Intensity)
	mask := clamp01(effect.MaskStrength)
	if intensity == 0 && mask == 0 {
		return false
	}
	period := effect.Thickness
	if period <= 0 {
		period = 3
	}
	// Aperture grille: each pixel column favors one triad channel and dims
	// the other two by mask.
	dim := 1 - mask
	triad := [3][3]float64{
		{1, dim, dim},
		{dim, 1, dim},
		{dim, dim, 1},
	}

	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		s := math.Sin(math.Pi * (float64(y-bounds.Min.Y) + 0.5) / period)
		factor := 1 - intensity*s*s
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gains := &triad[(x-bounds.Min.X)%3]
			pixel := pixelAt(src, x, y)
			pixel.r *= factor * gains[0]
			pixel.g *= factor * gains[1]
			pixel.b *= factor * gains[2]
			setPixel(dst, x, y, pixel)
		}
	}
	return true
}

func applyRadialBlur(src *image.RGBA, effect render.RadialBlur, camera render.Camera) *image.RGBA {
	if clamp01(effect.Strength) == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyRadialBlurInto(dst, src, effect, camera)
	return dst
}

// radialBlurTaps is the fixed sample count of the streak; each pixel averages
// this many bilinear taps toward the focus point.
const radialBlurTaps = 8

func applyRadialBlurInto(dst, src *image.RGBA, effect render.RadialBlur, camera render.Camera) bool {
	strength := clamp01(effect.Strength)
	if strength == 0 {
		return false
	}

	center := camera.WorldToScreen(effect.Center)
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			deltaX := (center.X - (float64(x) + 0.5)) * strength * 0.15
			deltaY := (center.Y - (float64(y) + 0.5)) * strength * 0.15
			var r, g, b, a float64
			for i := 0; i < radialBlurTaps; i++ {
				t := float64(i) / (radialBlurTaps - 1)
				sample := sampleBilinear(src, float64(x)+deltaX*t, float64(y)+deltaY*t)
				r += sample.r
				g += sample.g
				b += sample.b
				a += sample.a
			}
			setPixel(dst, x, y, postPixel{
				r: r / radialBlurTaps, g: g / radialBlurTaps,
				b: b / radialBlurTaps, a: a / radialBlurTaps,
			})
		}
	}
	return true
}

func applyShockwave(src *image.RGBA, effect render.Shockwave, camera render.Camera) *image.RGBA {
	if effect.Width == 0 || effect.Strength == 0 {
		return src
	}

	width := math.Abs(camera.WorldScalarToScreen(effect.Width))
	if width == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyShockwaveInto(dst, src, effect, camera)
	return dst
}

func applyShockwaveInto(dst, src *image.RGBA, effect render.Shockwave, camera render.Camera) bool {
	if effect.Width == 0 || effect.Strength == 0 {
		return false
	}

	center := camera.WorldToScreen(effect.Center)
	radius := math.Abs(camera.WorldScalarToScreen(effect.Radius))
	width := math.Abs(camera.WorldScalarToScreen(effect.Width))
	if width == 0 {
		return false
	}

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
	return true
}

func applyLensRain(src *image.RGBA, effect render.LensRain, time float64) *image.RGBA {
	density := clamp01(effect.Density)
	if density == 0 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	applyLensRainInto(dst, src, effect, time)
	return dst
}

func applyLensRainInto(dst, src *image.RGBA, effect render.LensRain, time float64) bool {
	density := clamp01(effect.Density)
	if density == 0 {
		return false
	}

	size := effect.Size
	if size <= 0 {
		size = 48
	}
	speed := effect.Speed
	if speed == 0 {
		speed = 0.15
	}

	bounds := src.Bounds()
	statics := [2]rainLayer{
		buildRainLayer(bounds, density, size*0.25, speed, effect.Seed+0x13, time, 0.45, 0.95, rainTwinkle),
		buildRainLayer(bounds, density*0.55, size*0.85, speed, effect.Seed+0x29, time, 0.80, 1.00, rainStatic),
	}
	erode := buildRainLayer(bounds, density*0.50, size*2.20, speed, effect.Seed+0x41, time, 1.25, 1.00, rainErode)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := pixelAt(src, x, y)
			for i := range statics {
				pixel = statics[i].apply(src, pixel, x, y, effect.Refraction)
			}
			pixel = erode.apply(src, pixel, x, y, effect.Refraction)
			setPixel(dst, x, y, pixel)
		}
	}
	return true
}

// rainMode selects the droplet lifecycle of a rainLayer.
type rainMode int

const (
	// rainStatic keeps the droplet fixed for the whole frame.
	rainStatic rainMode = iota
	// rainTwinkle keeps the droplet fixed but glints slowly over time.
	rainTwinkle
	// rainErode gives the droplet a lifecycle: it splashes in, holds, then
	// gradually shrinks away. Each lifecycle picks a new position inside the
	// cell, so drops do not repeat in place.
	rainErode
)

// rainCell holds one static droplet, precomputed once per frame instead of
// re-deriving it from the hash at every pixel.
type rainCell struct {
	active     bool
	centerX    float64
	centerY    float64
	radius     float64
	alpha      float64
	lightScale float64
	shape      float64
}

// rainLayer is a static droplet grid: cells map pixel columns and rows to
// cell indices so the per-pixel loop only scans the 3x3 neighborhood.
type rainLayer struct {
	refrScale float64
	minX      int
	minY      int
	cols      []int
	rows      []int
	cells     []rainCell
	colCount  int
	rowCount  int
}

// buildRainLayer evaluates the droplet hash once per grid cell. In erode
// mode the position hash is re-keyed with the lifecycle index, so every
// cycle lands the droplet somewhere new inside the cell.
func buildRainLayer(bounds image.Rectangle, density, size, speed float64, seed uint64, time, refrScale, opacity float64, mode rainMode) rainLayer {
	layer := rainLayer{refrScale: refrScale, minX: bounds.Min.X, minY: bounds.Min.Y}

	layer.cols = make([]int, bounds.Dx())
	var colCells []float64
	for i := range layer.cols {
		cellX := math.Floor(float64(bounds.Min.X+i) / size)
		if len(colCells) == 0 || cellX != colCells[len(colCells)-1] {
			colCells = append(colCells, cellX)
		}
		layer.cols[i] = len(colCells) - 1
	}

	layer.rows = make([]int, bounds.Dy())
	var rowCells []float64
	for i := range layer.rows {
		cellY := math.Floor(float64(bounds.Min.Y+i) / size)
		if len(rowCells) == 0 || cellY != rowCells[len(rowCells)-1] {
			rowCells = append(rowCells, cellY)
		}
		layer.rows[i] = len(rowCells) - 1
	}

	layer.colCount = len(colCells)
	layer.rowCount = len(rowCells)
	layer.cells = make([]rainCell, len(rowCells)*len(colCells))
	for r, cellY := range rowCells {
		for c, cellX := range colCells {
			hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(seed)})
			h := [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
			if h[0] >= density {
				continue
			}
			radiusScale := 1.0
			alpha := opacity
			lightScale := 1.0
			if mode == rainErode {
				tCycle := time*speed*(0.6+0.8*h[1]) + h[0]
				phase := fract(tCycle)
				grow := smoothstep(0, 0.07, phase)
				shrink := 1 - smoothstep(0.40, 0.97, phase)
				radiusScale = grow * shrink
				if radiusScale <= 0 {
					continue
				}
				cellSeed := seed + uint64(uint32(int32(math.Floor(tCycle))))
				hash = particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(cellSeed)})
				h = [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
				alpha *= shrink
				lightScale = 0.3 + 0.7*shrink
			}
			if mode == rainTwinkle {
				alpha *= 0.85 + 0.15*math.Sin(time*(1+2*h[2])+h[0]*6.2832)
			}
			radius := size * (0.30 + 0.15*h[0]) * radiusScale
			if radius < 0.5 {
				continue
			}
			layer.cells[r*layer.colCount+c] = rainCell{
				active:     true,
				centerX:    (cellX + h[1]) * size,
				centerY:    (cellY + h[2]) * size,
				radius:     radius,
				alpha:      alpha,
				lightScale: lightScale,
				shape:      fract(h[0] * 61.8034),
			}
		}
	}
	return layer
}

// apply blends every droplet of the 3x3 cell neighborhood into the pixel.
// Scanning the neighborhood is what keeps droplets centered near a cell
// border whole instead of sliced at the boundary.
func (l *rainLayer) apply(src *image.RGBA, pixel postPixel, x, y int, refraction float64) postPixel {
	col := l.cols[x-l.minX]
	row := l.rows[y-l.minY]
	for dr := -1; dr <= 1; dr++ {
		r := row + dr
		if r < 0 || r >= l.rowCount {
			continue
		}
		for dc := -1; dc <= 1; dc++ {
			c := col + dc
			if c < 0 || c >= l.colCount {
				continue
			}
			cell := &l.cells[r*l.colCount+c]
			if !cell.active {
				continue
			}
			pixel = applyRainDrop(src, pixel, x, y, cell.centerX, cell.centerY,
				cell.radius, cell.radius, refraction*l.refrScale, cell.alpha, cell.lightScale, cell.shape)
		}
	}
	return pixel
}

// applyRainDrop shades one dome-shaped droplet: the background bends inward
// through the dome (strong at the rim), with a thin dark edge and a small
// specular highlight. Zero refraction keeps the pixel sample local (N1).
// lightScale flattens the dome lighting as an eroding droplet dries out.
func applyRainDrop(src *image.RGBA, pixel postPixel, x, y int, centerX, centerY, radiusX, radiusY, refraction, opacity, lightScale, shape float64) postPixel {
	deltaX := float64(x) + 0.5 - centerX
	deltaY := float64(y) + 0.5 - centerY
	nx := deltaX / radiusX
	ny := deltaY / radiusY
	// Gravity asymmetry: the drop bulges below its center and thins above it.
	if ny > 0 {
		ny *= 0.82
	} else {
		ny *= 1.12
	}
	// Angular wobble breaks the perfect circle; shape picks the per-drop phase.
	angle := math.Atan2(ny, nx)
	wobble := 1 + 0.09*math.Sin(3*angle+shape*6.2832) + 0.05*math.Sin(5*angle+shape*12.9)
	nx *= wobble
	ny *= wobble
	d2 := nx*nx + ny*ny
	if d2 >= 1 {
		return pixel
	}

	d := math.Sqrt(d2)
	dome := math.Sqrt(1 - d2)
	mask := smoothstep(1, 0.85, d) * opacity
	if mask <= 0 {
		return pixel
	}

	result := pixel
	if refraction > 0 {
		bend := refraction * (0.45 + 0.55*d/math.Max(dome, 0.25))
		result = sampleBilinear(src, float64(x)-deltaX*bend, float64(y)-deltaY*bend)
	}

	shade := 1 - 0.26*smoothstep(0.7, 1, d)
	highlight := smoothstep(0.5, 0, math.Hypot(nx+0.30, ny+0.32))
	light := (dome*0.05 + highlight*0.16) * lightScale
	result.r = result.r*shade + light
	result.g = result.g*shade + light
	result.b = result.b*shade + light
	pixel.r += (result.r - pixel.r) * mask
	pixel.g += (result.g - pixel.g) * mask
	pixel.b += (result.b - pixel.b) * mask
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
	i := img.PixOffset(x, y)
	return postPixel{
		r: float64(img.Pix[i]) / 255,
		g: float64(img.Pix[i+1]) / 255,
		b: float64(img.Pix[i+2]) / 255,
		a: float64(img.Pix[i+3]) / 255,
	}
}

func setPixel(img *image.RGBA, x, y int, p postPixel) {
	i := img.PixOffset(x, y)
	img.Pix[i] = toByte(clamp01(p.r))
	img.Pix[i+1] = toByte(clamp01(p.g))
	img.Pix[i+2] = toByte(clamp01(p.b))
	img.Pix[i+3] = toByte(clamp01(p.a))
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

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
	layers := [3]rainLayer{
		buildRainLayer(bounds, density*0.70, size*0.70, speed*1.20,
			effect.Refraction*0.70, effect.Seed+0x13, time, 0.18, 0.28, 0.85),
		buildRainLayer(bounds, density*0.45, size*1.35, speed*0.80,
			effect.Refraction, effect.Seed+0x29, time, 0.22, 0.34, 0.90),
		buildRainLayer(bounds, density*0.18, size*3.00, speed*0.55,
			effect.Refraction*1.25, effect.Seed+0x41, time, 0.13, 0.42, 0.65),
	}

	var rowBase [3]int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for i := range layers {
			rowBase[i] = layers[i].rows[y-bounds.Min.Y] * layers[i].colCount
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := pixelAt(src, x, y)
			for i := range layers {
				layer := &layers[i]
				cell := &layer.cells[rowBase[i]+layer.cols[x-bounds.Min.X]]
				if cell.active {
					pixel = layer.applyCell(src, pixel, cell, x, y)
				}
			}
			setPixel(dst, x, y, pixel)
		}
	}
	return true
}

// rainCell holds the droplet a grid cell contains, precomputed once per frame
// instead of re-deriving it from the hash at every pixel.
type rainCell struct {
	active  bool
	centerX float64
	centerY float64
	radiusX float64
	radiusY float64
	// skew is the X shear of the droplet shape, (h1-0.5)*0.20 — the same
	// left-associated grouping the per-pixel formula used, so results match.
	skew float64
}

// rainLayer holds one droplet layer's cell grid and the per-pixel cell index
// maps for a frame.
type rainLayer struct {
	size       float64
	refraction float64
	opacity    float64
	cols       []int
	rows       []int
	cells      []rainCell
	colCount   int
}

// buildRainLayer evaluates the droplet hash once per grid cell. The cell
// coordinate keeps the exact expression floor(coord/size) so the grid
// boundaries land on the same pixels as the previous per-pixel evaluation.
func buildRainLayer(bounds image.Rectangle, density, size, speed, refraction float64, seed uint64, time, radiusXScale, radiusYScale, opacity float64) rainLayer {
	layer := rainLayer{size: size, refraction: refraction, opacity: opacity}

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
	layer.cells = make([]rainCell, len(rowCells)*len(colCells))
	for r, cellY := range rowCells {
		for c, cellX := range colCells {
			hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(seed)})
			h := [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
			if h[0] >= density {
				continue
			}
			fall := fract(h[0] - time*speed*(0.6+0.8*h[0]))
			layer.cells[r*layer.colCount+c] = rainCell{
				active:  true,
				centerX: (cellX + h[1]) * size,
				centerY: (cellY + fall) * size,
				radiusX: size * (radiusXScale + 0.10*h[2]),
				radiusY: size * (radiusYScale + 0.18*h[2]),
				skew:    (h[1] - 0.5) * 0.20,
			}
		}
	}
	return layer
}

// applyCell blends one droplet into the pixel. The early exits are exact:
// |dy| >= radiusY forces shapeY >= 1, and |dx| >= 1.2*radiusX forces
// shapeX >= 1.1 - |skew| >= 1 (|skew| <= 0.10), so the mask is provably zero
// in both cases and the previous per-pixel code returned unchanged too.
func (l *rainLayer) applyCell(src *image.RGBA, pixel postPixel, cell *rainCell, x, y int) postPixel {
	deltaY := float64(y) + 0.5 - cell.centerY
	if deltaY >= cell.radiusY || -deltaY >= cell.radiusY {
		return pixel
	}
	deltaX := float64(x) + 0.5 - cell.centerX
	if deltaX >= 1.2*cell.radiusX || -deltaX >= 1.2*cell.radiusX {
		return pixel
	}

	delta := math2d.Vec2{X: deltaX, Y: deltaY}
	distance := delta.Len()
	shapeX := delta.X/cell.radiusX - cell.skew*(delta.Y/cell.radiusY)
	shapeY := delta.Y / cell.radiusY
	normalizedDistance := math.Hypot(shapeX, shapeY)
	mask := smoothstep(1, 0.80, normalizedDistance)
	if mask == 0 {
		return pixel
	}

	result := pixelAt(src, x, y)
	if l.refraction > 0 && distance > 0 {
		offset := delta.Mul((1 - normalizedDistance) * l.refraction * l.size * 0.65 / distance)
		result = sampleBilinear(src, float64(x)-offset.X, float64(y)-offset.Y)
	}

	highlightDistance := math.Hypot(shapeX+0.28, shapeY+0.34)
	highlight := smoothstep(0.54, 0, highlightDistance) * mask
	rim := smoothstep(0.48, 0.92, normalizedDistance) *
		(1 - smoothstep(0.92, 1, normalizedDistance))
	shadow := 0.10*mask + 0.48*rim
	result.r *= 1 - shadow
	result.g *= 1 - shadow
	result.b *= 1 - shadow
	result.r += highlight * 0.16
	result.g += highlight * 0.19
	result.b += highlight * 0.24
	amount := mask * l.opacity
	pixel.r += (result.r - pixel.r) * amount
	pixel.g += (result.g - pixel.g) * amount
	pixel.b += (result.b - pixel.b) * amount
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

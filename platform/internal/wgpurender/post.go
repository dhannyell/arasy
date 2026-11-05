//go:build ((windows || linux || darwin) && (amd64 || arm64)) || js

package wgpurender

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/render/halftone"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/webgpu/wgpu"
)

const (
	postBlit         = "blit"
	postFinalBlit    = "final-blit"
	postVignette     = "vignette"
	postColorGrade   = "color-grade"
	postPixelate     = "pixelate"
	postChromatic    = "chromatic"
	postBloomBright  = "bloom-bright"
	postBloomDown    = "bloom-down"
	postBloomUp      = "bloom-up"
	postBloomCombine = "bloom-combine"
	postShockwave    = "shockwave"
	postLensRain     = "lens-rain"
	postBlurDown     = "blur-down"
	postBlurUp       = "blur-up"
	postBlurCombine  = "blur-combine"
	postFade         = "fade"
	postCircleWipe   = "circle-wipe"
	postDissolve     = "dissolve"
	postGrain        = "grain"
	postScanlines    = "scanlines"
	postRadialBlur   = "radial-blur"
	postHalftone     = "halftone"
	postFocusCombine = "focus-combine"
)

type postTarget struct {
	texture *wgpu.Texture
	view    *wgpu.TextureView
	width   int
	height  int
}

// Stable identifiers for the sampled post targets, used as bind group cache
// keys: the scene ping-pong pair, then one slot per bloom level.
const (
	postSourceSceneA = 0
	postSourceSceneB = 1
	postSourceBloom  = 2
)

// postBindKey identifies one cached post bind group. bloom is the second
// sampled target of the combine pass (-1 elsewhere); uniform is the
// postUniforms slot (-1 for the uniform-less blit).
type postBindKey struct {
	pipeline string
	source   int
	bloom    int
	uniform  int
}

func (r *Renderer) releasePostBindGroups() {
	for _, group := range r.postBindGroups {
		group.Release()
	}
	clear(r.postBindGroups)
}

func (r *Renderer) createPostResources() error {
	sampler, err := r.device.Device.TryCreateSampler(&wgpu.SamplerDescriptor{
		Label:         "Arasy post sampler",
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		MaxAnisotropy: 1,
	})
	if err != nil {
		return fmt.Errorf("create post sampler: %w", err)
	}
	r.postSampler = sampler
	r.postPipelines = make(map[string]*wgpu.RenderPipeline)
	for key, spec := range map[string]struct {
		label  string
		source string
		entry  string
	}{
		postBlit:         {"post blit", shaders.PostBlitWGSL, "fs_main"},
		postFinalBlit:    {"post final blit", shaders.PostBlitWGSL, "fs_main"},
		postVignette:     {"post vignette", shaders.PostVignetteWGSL, "fs_main"},
		postColorGrade:   {"post color grade", shaders.PostColorGradeWGSL, "fs_main"},
		postPixelate:     {"post pixelate", shaders.PostPixelateWGSL, "fs_main"},
		postChromatic:    {"post chromatic", shaders.PostChromaticWGSL, "fs_main"},
		postBloomBright:  {"post bloom bright", shaders.PostBloomWGSL, "fs_bright"},
		postBloomDown:    {"post bloom down", shaders.PostBloomWGSL, "fs_down"},
		postBloomUp:      {"post bloom up", shaders.PostBloomWGSL, "fs_up"},
		postBloomCombine: {"post bloom combine", shaders.PostBloomCombineWGSL, "fs_main"},
		postShockwave:    {"post shockwave", shaders.PostShockwaveWGSL, "fs_main"},
		postLensRain:     {"post lens rain", shaders.PostLensRainWGSL, "fs_main"},
		// Blur reuses the bloom pyramid's fs_down/fs_up entry points (same
		// taps and weights) and only owns the final sharp/blurred mix.
		postBlurDown:    {"post blur down", shaders.PostBloomWGSL, "fs_down"},
		postBlurUp:      {"post blur up", shaders.PostBloomWGSL, "fs_up"},
		postBlurCombine: {"post blur combine", shaders.PostBlurWGSL, "fs_main"},
		postFade:        {"post fade", shaders.PostFadeWGSL, "fs_main"},
		postCircleWipe:  {"post circle wipe", shaders.PostCircleWipeWGSL, "fs_main"},
		postDissolve:    {"post dissolve", shaders.PostDissolveWGSL, "fs_main"},
		postGrain:       {"post grain", shaders.PostGrainWGSL, "fs_main"},
		postScanlines:   {"post scanlines", shaders.PostScanlinesWGSL, "fs_main"},
		postRadialBlur:  {"post radial blur", shaders.PostRadialBlurWGSL, "fs_main"},
		postHalftone:    {"post halftone", shaders.PostHalftoneWGSL, "fs_main"},
		// FocusBlur shares Blur's pyramid and only owns its weighted mix.
		postFocusCombine: {"post focus combine", shaders.PostFocusBlurWGSL, "fs_main"},
	} {
		samples := uint32(1)
		if key == postFinalBlit {
			samples = r.msaaSamples
		}
		pipeline, createErr := r.createPipelineEntry(spec.label, spec.source, spec.entry, r.device.Config.Format, samples, nil, nil, nil)
		if createErr != nil {
			r.releasePostPipelines()
			releaseSampler(&r.postSampler)
			return createErr
		}
		r.postPipelines[key] = pipeline
	}
	return nil
}

func (r *Renderer) releasePostPipelines() {
	for _, pipeline := range r.postPipelines {
		if pipeline != nil {
			pipeline.Release()
		}
	}
	r.postPipelines = nil
}

func (r *Renderer) disablePost(err error) {
	if r.postDisabled {
		return
	}
	r.postDisabled = true
	r.log.Warn("post-processing disabled; drawing without chain", "err", err)
	r.releasePostBindGroups()
	r.releasePostTarget(&r.postSceneA)
	r.releasePostTarget(&r.postSceneB)
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = nil
}

// postLUTOffset is where the halftone threshold table starts in the
// post uniform, after the 64 bytes of scalar parameters every effect shares;
// postUniformSize fits the table for the widest cell. Only Halftone and
// the FocusBlur mix upload past the scalar block.
const (
	postLUTOffset   = 64
	postScalarSize  = postLUTOffset
	postFocusSize   = postLUTOffset + 8*render.FocusBlurMaxPoints
	postUniformSize = postLUTOffset + 4*halftone.MaxCell*halftone.MaxCell
)

func (r *Renderer) ensurePostUniforms(count int) error {
	for len(r.postUniforms) < count {
		buffer, err := r.newBuffer("post uniform", postUniformSize, wgpu.BufferUsageUniform|wgpu.BufferUsageCopyDst)
		if err != nil {
			return err
		}
		r.postUniforms = append(r.postUniforms, buffer)
	}
	return nil
}

func (r *Renderer) ensurePostTargets(width, height int) error {
	if r.postWidth == width && r.postHeight == height && r.postSceneA.texture != nil {
		return nil
	}
	r.releasePostTarget(&r.postSceneA)
	r.releasePostTarget(&r.postSceneB)
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = nil
	r.releasePostBindGroups()
	create := func(w, h int) (postTarget, error) {
		texture, err := r.device.Device.TryCreateTexture(&wgpu.TextureDescriptor{
			Label:         "Arasy post target",
			Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
			Dimension:     wgpu.TextureDimension2D,
			Size:          wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
			Format:        r.device.Config.Format,
			MipLevelCount: 1,
			SampleCount:   1,
		})
		if err != nil {
			return postTarget{}, fmt.Errorf("create post target: %w", err)
		}
		view, err := texture.TryCreateView(nil)
		if err != nil {
			texture.Release()
			return postTarget{}, fmt.Errorf("create post target view: %w", err)
		}
		return postTarget{texture: texture, view: view, width: w, height: h}, nil
	}
	var err error
	if r.postSceneA, err = create(width, height); err != nil {
		return err
	}
	if r.postSceneB, err = create(width, height); err != nil {
		r.releasePostTarget(&r.postSceneA)
		return err
	}
	r.postWidth, r.postHeight = width, height
	return nil
}

func (r *Renderer) ensurePostBloom(passes int) error {
	// Grow-only: Bloom and Blur share this pyramid inside one frame, so
	// recreating or shrinking it here would destroy textures a pass already
	// encoded on this frame's encoder still references. Stale sizes are
	// handled by ensurePostTargets, which releases the pyramid on resize.
	if len(r.postBloom) >= passes+1 {
		return nil
	}
	width, height := r.postWidth, r.postHeight
	for range r.postBloom {
		width, height = max(1, (width+1)/2), max(1, (height+1)/2)
	}
	for level := len(r.postBloom); level <= passes; level++ {
		texture, err := r.device.Device.TryCreateTexture(&wgpu.TextureDescriptor{
			Label:         "Arasy bloom level",
			Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
			Dimension:     wgpu.TextureDimension2D,
			Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
			Format:        r.device.Config.Format,
			MipLevelCount: 1,
			SampleCount:   1,
		})
		if err != nil {
			return fmt.Errorf("create bloom level %d: %w", level, err)
		}
		view, err := texture.TryCreateView(nil)
		if err != nil {
			texture.Release()
			return fmt.Errorf("create bloom level %d view: %w", level, err)
		}
		r.postBloom = append(r.postBloom, postTarget{texture: texture, view: view, width: width, height: height})
		width, height = max(1, (width+1)/2), max(1, (height+1)/2)
	}
	return nil
}

func (r *Renderer) releasePostTarget(target *postTarget) {
	if target.view != nil {
		target.view.Release()
	}
	if target.texture != nil {
		target.texture.Destroy()
		target.texture.Release()
	}
	*target = postTarget{}
}

func postF32(dst []byte, index int, value float64) {
	binary.LittleEndian.PutUint32(dst[index:], math.Float32bits(float32(value)))
}

func postU32(dst []byte, index int, value uint32) {
	binary.LittleEndian.PutUint32(dst[index:], value)
}

func postBloomPasses(effect render.Bloom) int {
	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	return max(1, min(5, passes))
}

// writePostUniform stages one effect's parameters into the pooled uniform
// buffer at index. The scratch is cleared first so unwritten fields keep the
// deterministic zero the per-call allocation used to give them.
func (r *Renderer) writePostUniform(index int, write func([]byte)) error {
	return r.writePostUniformSized(index, postScalarSize, write)
}

// writePostUniformSized uploads only the first size bytes. A shader never
// reads past its own Params, so the stale tail of the buffer is harmless.
func (r *Renderer) writePostUniformSized(index, size int, write func([]byte)) error {
	if err := r.ensurePostUniforms(index + 1); err != nil {
		return err
	}
	data := r.postUniformScratch[:size]
	clear(data)
	write(data)
	if err := r.device.Queue.TryWriteBuffer(r.postUniforms[index], 0, data); err != nil {
		return fmt.Errorf("write post uniform: %w", err)
	}
	return nil
}

// postParams writes the effect's uniform at the given slot and returns the
// pipeline that consumes it, or "" for an effect this backend does not encode.
func (r *Renderer) postParams(index int, effect render.PostEffect, camera render.Camera, time float64, width, height int) (string, error) {
	switch effect := effect.(type) {
	case render.Vignette:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Color.R)
			postF32(dst, 4, effect.Color.G)
			postF32(dst, 8, effect.Color.B)
			postF32(dst, 12, effect.Color.A)
			postF32(dst, 16, effect.Strength)
			postF32(dst, 20, effect.Softness)
		})
		return postVignette, err
	case render.ColorGrade:
		tint := effect.Tint
		if tint == (render.Color{}) {
			tint = render.RGBA(1, 1, 1, 1)
		}
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Brightness)
			postF32(dst, 4, effect.Contrast)
			postF32(dst, 8, effect.Saturation)
			postF32(dst, 16, tint.R)
			postF32(dst, 20, tint.G)
			postF32(dst, 24, tint.B)
		})
		return postColorGrade, err
	case render.Pixelate:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Size)
			postF32(dst, 4, float64(width))
			postF32(dst, 8, float64(height))
		})
		return postPixelate, err
	case render.ChromaticAberration:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Strength)
			postF32(dst, 4, float64(width))
			postF32(dst, 8, float64(height))
		})
		return postChromatic, err
	case render.Shockwave:
		center := camera.WorldToScreen(effect.Center)
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, center.X)
			postF32(dst, 4, center.Y)
			postF32(dst, 8, math.Abs(camera.WorldScalarToScreen(effect.Radius)))
			postF32(dst, 12, math.Abs(camera.WorldScalarToScreen(effect.Width)))
			postF32(dst, 16, effect.Strength)
			postF32(dst, 20, float64(width))
			postF32(dst, 24, float64(height))
		})
		return postShockwave, err
	case render.LensRain:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Density)
			postF32(dst, 4, effect.Size)
			postF32(dst, 8, effect.Speed)
			postF32(dst, 12, effect.Refraction)
			postF32(dst, 16, time)
			postF32(dst, 20, float64(width))
			postF32(dst, 24, float64(height))
			postU32(dst, 28, uint32(effect.Seed))
		})
		return postLensRain, err
	case render.Fade:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Color.R)
			postF32(dst, 4, effect.Color.G)
			postF32(dst, 8, effect.Color.B)
			postF32(dst, 12, effect.Color.A)
			postF32(dst, 16, effect.Amount)
		})
		return postFade, err
	case render.CircleWipe:
		center := camera.WorldToScreen(effect.Center)
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Color.R)
			postF32(dst, 4, effect.Color.G)
			postF32(dst, 8, effect.Color.B)
			postF32(dst, 12, effect.Color.A)
			postF32(dst, 16, center.X)
			postF32(dst, 20, center.Y)
			postF32(dst, 24, effect.Progress)
			postF32(dst, 28, effect.Softness)
			postF32(dst, 32, float64(width))
			postF32(dst, 36, float64(height))
		})
		return postCircleWipe, err
	case render.Dissolve:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Color.R)
			postF32(dst, 4, effect.Color.G)
			postF32(dst, 8, effect.Color.B)
			postF32(dst, 12, effect.Color.A)
			postF32(dst, 16, effect.Amount)
			postF32(dst, 20, effect.CellSize)
			postF32(dst, 36, float64(width))
			postF32(dst, 40, float64(height))
			postU32(dst, 44, uint32(effect.Seed))
		})
		return postDissolve, err
	case render.FilmGrain:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Intensity)
			postF32(dst, 4, effect.Size)
			postF32(dst, 20, float64(width))
			postF32(dst, 24, float64(height))
			postU32(dst, 28, uint32(effect.Seed)+uint32(time*60))
		})
		return postGrain, err
	case render.Scanlines:
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, effect.Intensity)
			postF32(dst, 4, effect.Thickness)
			postF32(dst, 8, effect.MaskStrength)
			postF32(dst, 20, float64(width))
			postF32(dst, 24, float64(height))
		})
		return postScanlines, err
	case render.RadialBlur:
		center := camera.WorldToScreen(effect.Center)
		err := r.writePostUniform(index, func(dst []byte) {
			postF32(dst, 0, center.X)
			postF32(dst, 4, center.Y)
			postF32(dst, 8, effect.Strength)
			postF32(dst, 20, float64(width))
			postF32(dst, 24, float64(height))
		})
		return postRadialBlur, err
	case render.Halftone:
		err := r.writePostUniformSized(index, postUniformSize, func(dst []byte) {
			postF32(dst, 0, effect.CellSize)
			postF32(dst, 4, float64(effect.Levels))
			postF32(dst, 8, float64(effect.Shape))
			postF32(dst, 12, effect.Strength)
			postF32(dst, 20, float64(width))
			postF32(dst, 24, float64(height))
			cell := halftone.Cell(effect.CellSize)
			postF32(dst, 16, float64(halftone.Pixel(effect.Pixel)))
			postF32(dst, 28, float64(cell))
			postF32(dst, 32, effect.Noise)
			postU32(dst, 36, halftone.NoiseTick(effect.Seed, effect.NoiseRate, time))
			for i, m := range halftone.Thresholds(effect.Shape, cell) {
				postF32(dst, postLUTOffset+4*i, float64(m))
			}
		})
		return postHalftone, err
	default:
		return "", nil
	}
}

func postBlurPasses(effect render.Blur) int {
	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	return max(1, min(5, passes))
}

// encodeBlur mirrors encodeBloom's pyramid but seeds level zero
// directly from the scene (no bright pass) and combines with a
// sharp/blurred mix instead of an additive combine.
func (r *Renderer) encodeBlur(encoder *wgpu.CommandEncoder, source postTarget, sourceID int, destination *wgpu.TextureView, effect render.Blur, index int) error {
	strength := clampPostStrength(effect.Strength)
	if strength == 0 {
		return nil
	}
	return r.encodePyramidBlur(encoder, source, sourceID, destination, postBlurPasses(effect), index,
		postBlurCombine, postScalarSize, func(data []byte) { postF32(data, 0, strength) })
}

// encodeFocusBlur is encodeBlur with the focus points, radius
// and falloff handed to a combine that weights the mix per pixel.
func (r *Renderer) encodeFocusBlur(encoder *wgpu.CommandEncoder, source postTarget, sourceID int, destination *wgpu.TextureView, effect render.FocusBlur, camera render.Camera, index int) error {
	strength := clampPostStrength(effect.Strength)
	if strength == 0 {
		return nil
	}
	points := effect.Points
	if len(points) > render.FocusBlurMaxPoints {
		points = points[:render.FocusBlurMaxPoints]
	}
	passes := postBlurPasses(render.Blur{Passes: effect.Passes})
	return r.encodePyramidBlur(encoder, source, sourceID, destination, passes, index,
		postFocusCombine, postFocusSize, func(data []byte) {
			half, rounding := focusRegion(effect, camera)
			postF32(data, 0, strength)
			postF32(data, 4, math.Abs(camera.WorldScalarToScreen(effect.Falloff)))
			postF32(data, 8, float64(len(points)))
			postF32(data, 12, rounding)
			postF32(data, 16, half[0])
			postF32(data, 20, float64(source.width))
			postF32(data, 24, float64(source.height))
			postF32(data, 28, half[1])
			for i, p := range points {
				at := camera.WorldToScreen(p)
				postF32(data, postLUTOffset+8*i, at.X)
				postF32(data, postLUTOffset+8*i+4, at.Y)
			}
		})
}

// focusRegion mirrors platform/raster/postfx.go focusRegion.
func focusRegion(effect render.FocusBlur, camera render.Camera) (half [2]float64, rounding float64) {
	half = [2]float64{
		math.Abs(camera.WorldScalarToScreen(effect.Extent.X)),
		math.Abs(camera.WorldScalarToScreen(effect.Extent.Y)),
	}
	rounding = math.Abs(camera.WorldScalarToScreen(effect.Rounding))
	if half[0] == 0 && half[1] == 0 {
		half = [2]float64{rounding, rounding}
	}
	rounding = math.Min(rounding, math.Min(half[0], half[1]))
	return half, rounding
}

// encodePyramidBlur runs the dual-Kawase pyramid over the scene and
// hands the sharp and blurred images to the named combine pipeline, whose
// uniform of combineSize bytes writeCombine fills.
func (r *Renderer) encodePyramidBlur(encoder *wgpu.CommandEncoder, source postTarget, sourceID int, destination *wgpu.TextureView, passes, index int, combine string, combineSize int, writeCombine func([]byte)) error {
	if err := r.ensurePostBloom(passes); err != nil {
		return err
	}
	// The first downsample reads the scene directly instead of blitting it
	// into level zero first: the output is identical and one fullscreen pass
	// cheaper. Level zero is still written by the final upsample below.
	srcView, srcID := source.view, sourceID
	srcWidth, srcHeight := source.width, source.height
	for level := 1; level <= passes; level++ {
		dst := r.postBloom[level]
		uniformIndex := index + level
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			postF32(data, 0, float64(srcWidth))
			postF32(data, 4, float64(srcHeight))
			postF32(data, 8, float64(dst.width))
			postF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, postBlurDown, srcID, srcView, dst.view, uniformIndex); err != nil {
			return err
		}
		srcView, srcID = dst.view, postSourceBloom+level
		srcWidth, srcHeight = dst.width, dst.height
	}
	for level := passes - 1; level >= 0; level-- {
		src, dst := r.postBloom[level+1], r.postBloom[level]
		uniformIndex := index + passes + 1 + (passes - 1 - level)
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			postF32(data, 0, float64(src.width))
			postF32(data, 4, float64(src.height))
			postF32(data, 8, float64(dst.width))
			postF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, postBlurUp, postSourceBloom+level+1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	combineIndex := index + 2*passes + 1
	if err := r.writePostUniformSized(combineIndex, combineSize, writeCombine); err != nil {
		return err
	}
	return r.encodePostCombine2(encoder, combine, sourceID, source.view, r.postBloom[0].view, destination, combineIndex)
}

// encodePostCombine2 is encodePostCombine generalized over the
// pipeline name, since Blur's combine shares the two-texture bind layout but
// a different pipeline than bloom's combine.
func (r *Renderer) encodePostCombine2(encoder *wgpu.CommandEncoder, pipelineName string, sourceID int, source, second, destination *wgpu.TextureView, uniformIndex int) error {
	pipeline := r.postPipelines[pipelineName]
	key := postBindKey{pipeline: pipelineName, source: sourceID, bloom: postSourceBloom, uniform: uniformIndex}
	bindGroup, err := r.cachedPostBindGroup(key, func() (*wgpu.BindGroup, error) {
		layout := pipeline.GetBindGroupLayout(0)
		defer layout.Release()
		return r.device.Device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: source}, {Binding: 1, TextureView: second}, {Binding: 2, Sampler: r.postSampler},
			{Binding: 3, Buffer: r.postUniforms[uniformIndex], Size: wgpu.WholeSize},
		}})
	})
	if err != nil {
		return err
	}
	pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: destination, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{A: 1},
	}}})
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, bindGroup, nil)
	pass.Draw(3, 1, 0, 0)
	if err := pass.TryEnd(); err != nil {
		pass.Release()
		return err
	}
	pass.Release()
	return nil
}

// clampPostStrength mirrors platform/raster/postfx.go clamp01 for the
// one Blur field the encode loop must gate on before spending a pyramid pass.
func clampPostStrength(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 1
	}
	return value
}

// cachedPostBindGroup returns the bind group for key, building it on the
// first use of that combination. The cache owns the group; it is released
// only when the targets are recreated or the renderer shuts down.
func (r *Renderer) cachedPostBindGroup(key postBindKey, build func() (*wgpu.BindGroup, error)) (*wgpu.BindGroup, error) {
	if group, ok := r.postBindGroups[key]; ok {
		return group, nil
	}
	if r.postBindGroups == nil {
		r.postBindGroups = map[postBindKey]*wgpu.BindGroup{}
	}
	group, err := build()
	if err != nil {
		return nil, err
	}
	r.postBindGroups[key] = group
	return group, nil
}

func (r *Renderer) postBindGroup(key postBindKey, source *wgpu.TextureView) (*wgpu.BindGroup, error) {
	return r.cachedPostBindGroup(key, func() (*wgpu.BindGroup, error) {
		pipeline := r.postPipelines[key.pipeline]
		layout := pipeline.GetBindGroupLayout(0)
		defer layout.Release()
		entries := []wgpu.BindGroupEntry{{Binding: 0, TextureView: source}, {Binding: 1, Sampler: r.postSampler}}
		if key.uniform >= 0 {
			entries = append(entries, wgpu.BindGroupEntry{Binding: 2, Buffer: r.postUniforms[key.uniform], Size: wgpu.WholeSize})
		}
		return r.device.Device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: entries})
	})
}

func (r *Renderer) encodePostPass(encoder *wgpu.CommandEncoder, pipelineName string, sourceID int, source, destination *wgpu.TextureView, uniformIndex int) error {
	pipeline := r.postPipelines[pipelineName]
	key := postBindKey{pipeline: pipelineName, source: sourceID, bloom: -1, uniform: uniformIndex}
	bindGroup, err := r.postBindGroup(key, source)
	if err != nil {
		return err
	}
	pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: destination, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{A: 1},
	}}})
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, bindGroup, nil)
	pass.Draw(3, 1, 0, 0)
	if err := pass.TryEnd(); err != nil {
		pass.Release()
		return err
	}
	pass.Release()
	return nil
}

func (r *Renderer) encodePostCombine(encoder *wgpu.CommandEncoder, sourceID int, source, bloom, destination *wgpu.TextureView, uniformIndex int) error {
	pipeline := r.postPipelines[postBloomCombine]
	key := postBindKey{pipeline: postBloomCombine, source: sourceID, bloom: postSourceBloom, uniform: uniformIndex}
	bindGroup, err := r.cachedPostBindGroup(key, func() (*wgpu.BindGroup, error) {
		layout := pipeline.GetBindGroupLayout(0)
		defer layout.Release()
		return r.device.Device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: source}, {Binding: 1, TextureView: bloom}, {Binding: 2, Sampler: r.postSampler},
			{Binding: 3, Buffer: r.postUniforms[uniformIndex], Size: wgpu.WholeSize},
		}})
	})
	if err != nil {
		return err
	}
	pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: destination, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{A: 1},
	}}})
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, bindGroup, nil)
	pass.Draw(3, 1, 0, 0)
	if err := pass.TryEnd(); err != nil {
		pass.Release()
		return err
	}
	pass.Release()
	return nil
}

func (r *Renderer) encodeBloom(encoder *wgpu.CommandEncoder, source postTarget, sourceID int, destination *wgpu.TextureView, effect render.Bloom, index int) error {
	if effect.Intensity == 0 {
		return nil
	}
	passes := postBloomPasses(effect)
	if err := r.ensurePostBloom(passes); err != nil {
		return err
	}
	if err := r.writePostUniform(index, func(dst []byte) { postF32(dst, 0, effect.Threshold) }); err != nil {
		return err
	}
	if err := r.encodePostPass(encoder, postBloomBright, sourceID, source.view, r.postBloom[0].view, index); err != nil {
		return err
	}
	for level := 1; level <= passes; level++ {
		src, dst := r.postBloom[level-1], r.postBloom[level]
		uniformIndex := index + level
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			postF32(data, 0, float64(src.width))
			postF32(data, 4, float64(src.height))
			postF32(data, 8, float64(dst.width))
			postF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, postBloomDown, postSourceBloom+level-1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	for level := passes - 1; level >= 0; level-- {
		src, dst := r.postBloom[level+1], r.postBloom[level]
		uniformIndex := index + passes + 1 + (passes - 1 - level)
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			postF32(data, 0, float64(src.width))
			postF32(data, 4, float64(src.height))
			postF32(data, 8, float64(dst.width))
			postF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, postBloomUp, postSourceBloom+level+1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	combineIndex := index + 2*passes + 1
	if err := r.writePostUniform(combineIndex, func(data []byte) { postF32(data, 0, effect.Intensity) }); err != nil {
		return err
	}
	return r.encodePostCombine(encoder, sourceID, source.view, r.postBloom[0].view, destination, combineIndex)
}

func (r *Renderer) encodePostFramePasses(runs []tess.StreamRun, uniformOffsets []uint32, plans []passPlan, surfaceView *wgpu.TextureView, camera render.Camera, chain render.PostChain, width, height int) error {
	if err := r.ensurePostTargets(width, height); err != nil {
		return err
	}
	encoder, err := r.device.Device.TryCreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("create post command encoder: %w", err)
	}
	defer encoder.Release()
	sampling := r.timeGPU && r.timing.sampleFrame()
	var timedCompute bool
	if r.particles != nil {
		sims := make([]particleSim, 0)
		for _, run := range runs {
			if run.Kind != tess.DrawRunParticles {
				continue
			}
			plan := plans[run.Pass]
			if sim, ok := r.particles.prepare(run.Particles, tess.CameraForLayer(plan.camera, plan.layerCameras, run.Layer), plan.width, plan.height); ok {
				sims = append(sims, sim)
			}
		}
		var emitWrites, simulateWrites *wgpu.PassTimestampWrites
		if sampling && len(sims) > 0 {
			emitWrites = r.timing.prepareWrites()
			simulateWrites = r.timing.simulateWrites()
			timedCompute = true
		}
		if err := r.particles.encode(encoder, sims, emitWrites, simulateWrites); err != nil {
			return err
		}
	}
	firstScreen, lastScreen := firstAndLastScreenPass(plans)
	for passIndex, plan := range plans {
		if plan.target != 0 {
			if err := r.encodeTargetPass(encoder, passIndex, plan, runs, uniformOffsets); err != nil {
				return err
			}
			continue
		}
		loadOp := wgpu.LoadOpLoad
		if passIndex == firstScreen {
			loadOp = wgpu.LoadOpClear
		}
		attachment := wgpu.RenderPassColorAttachment{
			View: r.msaaView, ResolveTarget: r.postSceneA.view, LoadOp: loadOp,
			StoreOp: wgpu.StoreOpStore, ClearValue: clearColor(r.clear),
		}
		if passIndex == lastScreen {
			attachment.StoreOp = wgpu.StoreOpDiscard
		}
		if r.msaaSamples <= 1 {
			attachment.View, attachment.ResolveTarget = r.postSceneA.view, nil
			attachment.StoreOp = wgpu.StoreOpStore
		}
		var writes *wgpu.PassTimestampWrites
		// The GPU render window ends at the last screen pass; the post
		// effect chain stays outside the measurement.
		if sampling {
			switch {
			case passIndex == firstScreen && passIndex == lastScreen:
				writes = r.timing.renderWrites()
			case passIndex == firstScreen:
				writes = r.timing.renderBeginWrites()
			case passIndex == lastScreen:
				writes = r.timing.renderEndWrites()
			}
		}
		pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{attachment}, TimestampWrites: writes})
		if err := r.drawPostPlanRuns(pass, runs, uniformOffsets, plans, passIndex, chain.BelowLayer, true); err != nil {
			pass.Release()
			return err
		}
		if err := pass.TryEnd(); err != nil {
			pass.Release()
			return err
		}
		pass.Release()
	}

	// Without MSAA and without runs above the chain, the last active effect
	// renders directly to the surface and the final blit pass disappears.
	lastActive := -1
	for i, effect := range chain.Effects {
		if postEffectEncodes(effect) {
			lastActive = i
		}
	}
	directFinal := r.msaaSamples <= 1 && lastActive >= 0 &&
		!hasPostRuns(runs, plans, chain.BelowLayer, false)

	source, destination := r.postSceneA, r.postSceneB
	sourceID, destID := postSourceSceneA, postSourceSceneB
	uniformIndex := 0
	for i, effect := range chain.Effects {
		destView := destination.view
		if directFinal && i == lastActive {
			destView = surfaceView
		}
		if bloom, ok := effect.(render.Bloom); ok {
			if bloom.Intensity == 0 {
				continue
			}
			if err := r.encodeBloom(encoder, source, sourceID, destView, bloom, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			sourceID, destID = destID, sourceID
			continue
		}
		if blur, ok := effect.(render.Blur); ok {
			if clampPostStrength(blur.Strength) == 0 {
				continue
			}
			if err := r.encodeBlur(encoder, source, sourceID, destView, blur, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			sourceID, destID = destID, sourceID
			continue
		}
		if focus, ok := effect.(render.FocusBlur); ok {
			if clampPostStrength(focus.Strength) == 0 {
				continue
			}
			if err := r.encodeFocusBlur(encoder, source, sourceID, destView, focus, camera, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			sourceID, destID = destID, sourceID
			continue
		}
		name, err := r.postParams(uniformIndex, effect, camera, chain.Time, width, height)
		if err != nil {
			return err
		}
		uniformIndex++
		if name == "" {
			continue
		}
		if err := r.encodePostPass(encoder, name, sourceID, source.view, destView, uniformIndex-1); err != nil {
			return err
		}
		source, destination = destination, source
		sourceID, destID = destID, sourceID
	}
	if !directFinal {
		final := wgpu.RenderPassColorAttachment{View: r.msaaView, ResolveTarget: surfaceView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpDiscard, ClearValue: clearColor(r.clear)}
		if r.msaaSamples <= 1 {
			final = wgpu.RenderPassColorAttachment{View: surfaceView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: clearColor(r.clear)}
		}
		finalPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{final}})
		blit := r.postPipelines[postFinalBlit]
		key := postBindKey{pipeline: postFinalBlit, source: sourceID, bloom: -1, uniform: -1}
		bindGroup, err := r.postBindGroup(key, source.view)
		if err != nil {
			finalPass.Release()
			return err
		}
		finalPass.SetPipeline(blit)
		finalPass.SetBindGroup(0, bindGroup, nil)
		finalPass.Draw(3, 1, 0, 0)
		last := -1
		scissor := scissorState{}
		for i, run := range runs {
			if !postRunVisible(run, len(plans)-1, chain.BelowLayer, false) {
				continue
			}
			plan := plans[run.Pass]
			effective := tess.CameraForLayer(plan.camera, plan.layerCameras, run.Layer)
			if err := r.drawRun(finalPass, run, i, uniformOffsets[i], effective, passViewport(plan.camera, plan.extra, width, height), &scissor, width, height, -1, false, false, &last); err != nil {
				finalPass.Release()
				return err
			}
		}
		if err := finalPass.TryEnd(); err != nil {
			finalPass.Release()
			return err
		}
		finalPass.Release()
	}
	slot := -1
	if sampling {
		slot = r.timing.encodeResolve(encoder, timedCompute)
	}
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish post command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.device.Queue.Submit(commandBuffer)
	if r.timeCPU {
		r.cpuFrame.submittedAt = time.Now()
	}
	if r.timeGPU {
		r.timing.readback(slot)
		r.timing.poll()
		r.timing.drain()
	}
	return nil
}

func (r *Renderer) drawPostPlanRuns(pass *wgpu.RenderPassEncoder, runs []tess.StreamRun, uniformOffsets []uint32, plans []passPlan, passIndex, belowLayer int, below bool) error {
	plan := plans[passIndex]
	scissor := scissorState{}
	last := -1
	for index, run := range runs {
		if run.Pass != passIndex || !postRunVisible(run, len(plans)-1, belowLayer, below) {
			continue
		}
		effective := tess.CameraForLayer(plan.camera, plan.layerCameras, run.Layer)
		if err := r.drawRun(pass, run, index, uniformOffsets[index], effective, passViewport(plan.camera, plan.extra, plan.width, plan.height), &scissor, plan.width, plan.height, -1, false, false, &last); err != nil {
			return err
		}
	}
	return nil
}

// postEffectEncodes mirrors the skip rules of the encode loop: a
// zero-intensity bloom and unknown effect types produce no pass.
func postEffectEncodes(effect render.PostEffect) bool {
	switch effect := effect.(type) {
	case render.Bloom:
		return effect.Intensity != 0
	case render.Blur:
		return clampPostStrength(effect.Strength) != 0
	case render.FocusBlur:
		return clampPostStrength(effect.Strength) != 0
	case render.Vignette, render.ColorGrade, render.Pixelate,
		render.ChromaticAberration, render.Shockwave, render.LensRain,
		render.Fade, render.CircleWipe, render.Dissolve, render.FilmGrain,
		render.Scanlines, render.RadialBlur, render.Halftone:
		return true
	default:
		return false
	}
}

func hasPostRuns(runs []tess.StreamRun, plans []passPlan, belowLayer int, below bool) bool {
	for _, run := range runs {
		if postRunVisible(run, len(plans)-1, belowLayer, below) {
			return true
		}
	}
	return false
}

func postRunVisible(run tess.StreamRun, basePass, belowLayer int, below bool) bool {
	if run.Pass < basePass {
		return below
	}
	if belowLayer == 0 {
		return below
	}
	if below {
		return run.Layer < belowLayer
	}
	return run.Layer >= belowLayer
}

//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/oliverbestmann/webgpu/wgpu"
)

const (
	nativePostBlit         = "blit"
	nativePostFinalBlit    = "final-blit"
	nativePostVignette     = "vignette"
	nativePostColorGrade   = "color-grade"
	nativePostPixelate     = "pixelate"
	nativePostChromatic    = "chromatic"
	nativePostBloomBright  = "bloom-bright"
	nativePostBloomDown    = "bloom-down"
	nativePostBloomUp      = "bloom-up"
	nativePostBloomCombine = "bloom-combine"
	nativePostShockwave    = "shockwave"
	nativePostLensRain     = "lens-rain"
	nativePostBlurDown     = "blur-down"
	nativePostBlurUp       = "blur-up"
	nativePostBlurCombine  = "blur-combine"
	nativePostFade         = "fade"
	nativePostCircleWipe   = "circle-wipe"
	nativePostDissolve     = "dissolve"
	nativePostGrain        = "grain"
	nativePostScanlines    = "scanlines"
	nativePostRadialBlur   = "radial-blur"
)

type nativePostTarget struct {
	texture *wgpu.Texture
	view    *wgpu.TextureView
	width   int
	height  int
}

// Stable identifiers for the sampled post targets, used as bind group cache
// keys: the scene ping-pong pair, then one slot per bloom level.
const (
	nativePostSourceSceneA = 0
	nativePostSourceSceneB = 1
	nativePostSourceBloom  = 2
)

// nativePostBindKey identifies one cached post bind group. bloom is the second
// sampled target of the combine pass (-1 elsewhere); uniform is the
// postUniforms slot (-1 for the uniform-less blit).
type nativePostBindKey struct {
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
	sampler, err := r.device.device.TryCreateSampler(&wgpu.SamplerDescriptor{
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
		nativePostBlit:         {"post blit", shaders.PostBlitWGSL, "fs_main"},
		nativePostFinalBlit:    {"post final blit", shaders.PostBlitWGSL, "fs_main"},
		nativePostVignette:     {"post vignette", shaders.PostVignetteWGSL, "fs_main"},
		nativePostColorGrade:   {"post color grade", shaders.PostColorGradeWGSL, "fs_main"},
		nativePostPixelate:     {"post pixelate", shaders.PostPixelateWGSL, "fs_main"},
		nativePostChromatic:    {"post chromatic", shaders.PostChromaticWGSL, "fs_main"},
		nativePostBloomBright:  {"post bloom bright", shaders.PostBloomWGSL, "fs_bright"},
		nativePostBloomDown:    {"post bloom down", shaders.PostBloomWGSL, "fs_down"},
		nativePostBloomUp:      {"post bloom up", shaders.PostBloomWGSL, "fs_up"},
		nativePostBloomCombine: {"post bloom combine", shaders.PostBloomCombineWGSL, "fs_main"},
		nativePostShockwave:    {"post shockwave", shaders.PostShockwaveWGSL, "fs_main"},
		nativePostLensRain:     {"post lens rain", shaders.PostLensRainWGSL, "fs_main"},
		// Blur reuses the bloom pyramid's fs_down/fs_up entry points (same
		// taps and weights) and only owns the final sharp/blurred mix.
		nativePostBlurDown:    {"post blur down", shaders.PostBloomWGSL, "fs_down"},
		nativePostBlurUp:      {"post blur up", shaders.PostBloomWGSL, "fs_up"},
		nativePostBlurCombine: {"post blur combine", shaders.PostBlurWGSL, "fs_main"},
		nativePostFade:        {"post fade", shaders.PostFadeWGSL, "fs_main"},
		nativePostCircleWipe:  {"post circle wipe", shaders.PostCircleWipeWGSL, "fs_main"},
		nativePostDissolve:    {"post dissolve", shaders.PostDissolveWGSL, "fs_main"},
		nativePostGrain:       {"post grain", shaders.PostGrainWGSL, "fs_main"},
		nativePostScanlines:   {"post scanlines", shaders.PostScanlinesWGSL, "fs_main"},
		nativePostRadialBlur:  {"post radial blur", shaders.PostRadialBlurWGSL, "fs_main"},
	} {
		samples := uint32(1)
		if key == nativePostFinalBlit {
			samples = r.msaaSamples
		}
		pipeline, createErr := r.createPipelineEntry(spec.label, spec.source, spec.entry, r.device.config.Format, samples, nil, nil)
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
	r.log.Warn("native post-processing disabled; drawing without chain", "err", err)
	r.releasePostBindGroups()
	r.releasePostTarget(&r.postSceneA)
	r.releasePostTarget(&r.postSceneB)
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = nil
}

func (r *Renderer) ensurePostUniforms(count int) error {
	for len(r.postUniforms) < count {
		buffer, err := r.newBuffer("post uniform", 64, wgpu.BufferUsageUniform|wgpu.BufferUsageCopyDst)
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
	create := func(w, h int) (nativePostTarget, error) {
		texture, err := r.device.device.TryCreateTexture(&wgpu.TextureDescriptor{
			Label:         "Arasy post target",
			Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
			Dimension:     wgpu.TextureDimension2D,
			Size:          wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
			Format:        r.device.config.Format,
			MipLevelCount: 1,
			SampleCount:   1,
		})
		if err != nil {
			return nativePostTarget{}, fmt.Errorf("create post target: %w", err)
		}
		view, err := texture.TryCreateView(nil)
		if err != nil {
			texture.Release()
			return nativePostTarget{}, fmt.Errorf("create post target view: %w", err)
		}
		return nativePostTarget{texture: texture, view: view, width: w, height: h}, nil
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
	if len(r.postBloom) == passes+1 && r.postBloom[0].width == r.postWidth && r.postBloom[0].height == r.postHeight {
		return nil
	}
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = make([]nativePostTarget, 0, passes+1)
	r.releasePostBindGroups()
	width, height := r.postWidth, r.postHeight
	for level := 0; level <= passes; level++ {
		texture, err := r.device.device.TryCreateTexture(&wgpu.TextureDescriptor{
			Label:         "Arasy bloom level",
			Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
			Dimension:     wgpu.TextureDimension2D,
			Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
			Format:        r.device.config.Format,
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
		r.postBloom = append(r.postBloom, nativePostTarget{texture: texture, view: view, width: width, height: height})
		width, height = max(1, (width+1)/2), max(1, (height+1)/2)
	}
	return nil
}

func (r *Renderer) releasePostTarget(target *nativePostTarget) {
	if target.view != nil {
		target.view.Release()
	}
	if target.texture != nil {
		target.texture.Destroy()
		target.texture.Release()
	}
	*target = nativePostTarget{}
}

func lastNativePostChain(commands []render.Command) (render.PostChain, bool) {
	var chain render.PostChain
	found := false
	for _, command := range commands {
		if candidate, ok := command.(render.PostChain); ok {
			chain, found = candidate, true
		}
	}
	return chain, found
}

func nativePostF32(dst []byte, index int, value float64) {
	binary.LittleEndian.PutUint32(dst[index:], math.Float32bits(float32(value)))
}

func nativePostU32(dst []byte, index int, value uint32) {
	binary.LittleEndian.PutUint32(dst[index:], value)
}

func nativePostBloomPasses(effect render.Bloom) int {
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
	if err := r.ensurePostUniforms(index + 1); err != nil {
		return err
	}
	data := r.postUniformScratch[:]
	clear(data)
	write(data)
	if err := r.device.queue.TryWriteBuffer(r.postUniforms[index], 0, data); err != nil {
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
			nativePostF32(dst, 0, effect.Color.R)
			nativePostF32(dst, 4, effect.Color.G)
			nativePostF32(dst, 8, effect.Color.B)
			nativePostF32(dst, 12, effect.Color.A)
			nativePostF32(dst, 16, effect.Strength)
			nativePostF32(dst, 20, effect.Softness)
		})
		return nativePostVignette, err
	case render.ColorGrade:
		tint := effect.Tint
		if tint == (render.Color{}) {
			tint = render.RGBA(1, 1, 1, 1)
		}
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Brightness)
			nativePostF32(dst, 4, effect.Contrast)
			nativePostF32(dst, 8, effect.Saturation)
			nativePostF32(dst, 16, tint.R)
			nativePostF32(dst, 20, tint.G)
			nativePostF32(dst, 24, tint.B)
		})
		return nativePostColorGrade, err
	case render.Pixelate:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Size)
			nativePostF32(dst, 4, float64(width))
			nativePostF32(dst, 8, float64(height))
		})
		return nativePostPixelate, err
	case render.ChromaticAberration:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Strength)
			nativePostF32(dst, 4, float64(width))
			nativePostF32(dst, 8, float64(height))
		})
		return nativePostChromatic, err
	case render.Shockwave:
		center := camera.WorldToScreen(effect.Center)
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, center.X)
			nativePostF32(dst, 4, center.Y)
			nativePostF32(dst, 8, math.Abs(camera.WorldScalarToScreen(effect.Radius)))
			nativePostF32(dst, 12, math.Abs(camera.WorldScalarToScreen(effect.Width)))
			nativePostF32(dst, 16, effect.Strength)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
		})
		return nativePostShockwave, err
	case render.LensRain:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Density)
			nativePostF32(dst, 4, effect.Size)
			nativePostF32(dst, 8, effect.Speed)
			nativePostF32(dst, 12, effect.Refraction)
			nativePostF32(dst, 16, time)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
			nativePostU32(dst, 28, uint32(effect.Seed))
		})
		return nativePostLensRain, err
	case render.Fade:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Color.R)
			nativePostF32(dst, 4, effect.Color.G)
			nativePostF32(dst, 8, effect.Color.B)
			nativePostF32(dst, 12, effect.Color.A)
			nativePostF32(dst, 16, effect.Amount)
		})
		return nativePostFade, err
	case render.CircleWipe:
		center := camera.WorldToScreen(effect.Center)
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Color.R)
			nativePostF32(dst, 4, effect.Color.G)
			nativePostF32(dst, 8, effect.Color.B)
			nativePostF32(dst, 12, effect.Color.A)
			nativePostF32(dst, 16, center.X)
			nativePostF32(dst, 20, center.Y)
			nativePostF32(dst, 24, effect.Progress)
			nativePostF32(dst, 28, effect.Softness)
			nativePostF32(dst, 32, float64(width))
			nativePostF32(dst, 36, float64(height))
		})
		return nativePostCircleWipe, err
	case render.Dissolve:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Color.R)
			nativePostF32(dst, 4, effect.Color.G)
			nativePostF32(dst, 8, effect.Color.B)
			nativePostF32(dst, 12, effect.Color.A)
			nativePostF32(dst, 16, effect.Amount)
			nativePostF32(dst, 20, effect.CellSize)
			nativePostF32(dst, 36, float64(width))
			nativePostF32(dst, 40, float64(height))
			nativePostU32(dst, 44, uint32(effect.Seed))
		})
		return nativePostDissolve, err
	case render.FilmGrain:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Intensity)
			nativePostF32(dst, 4, effect.Size)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
			nativePostU32(dst, 28, uint32(effect.Seed)+uint32(time*60))
		})
		return nativePostGrain, err
	case render.Scanlines:
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Intensity)
			nativePostF32(dst, 4, effect.Thickness)
			nativePostF32(dst, 8, effect.MaskStrength)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
		})
		return nativePostScanlines, err
	case render.RadialBlur:
		center := camera.WorldToScreen(effect.Center)
		err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, center.X)
			nativePostF32(dst, 4, center.Y)
			nativePostF32(dst, 8, effect.Strength)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
		})
		return nativePostRadialBlur, err
	default:
		return "", nil
	}
}

func nativePostBlurPasses(effect render.Blur) int {
	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	return max(1, min(5, passes))
}

// encodeNativeBlur mirrors encodeNativeBloom's pyramid but seeds level zero
// directly from the scene (no bright pass) and combines with a
// sharp/blurred mix instead of an additive combine.
func (r *Renderer) encodeNativeBlur(encoder *wgpu.CommandEncoder, source nativePostTarget, sourceID int, destination *wgpu.TextureView, effect render.Blur, index int) error {
	strength := nativeClampPostStrength(effect.Strength)
	if strength == 0 {
		return nil
	}
	passes := nativePostBlurPasses(effect)
	if err := r.ensurePostBloom(passes); err != nil {
		return err
	}
	if err := r.encodePostPass(encoder, nativePostBlit, sourceID, source.view, r.postBloom[0].view, -1); err != nil {
		return err
	}
	for level := 1; level <= passes; level++ {
		src, dst := r.postBloom[level-1], r.postBloom[level]
		uniformIndex := index + level
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			nativePostF32(data, 0, float64(src.width))
			nativePostF32(data, 4, float64(src.height))
			nativePostF32(data, 8, float64(dst.width))
			nativePostF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, nativePostBlurDown, nativePostSourceBloom+level-1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	for level := passes - 1; level >= 0; level-- {
		src, dst := r.postBloom[level+1], r.postBloom[level]
		uniformIndex := index + passes + 1 + (passes - 1 - level)
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			nativePostF32(data, 0, float64(src.width))
			nativePostF32(data, 4, float64(src.height))
			nativePostF32(data, 8, float64(dst.width))
			nativePostF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, nativePostBlurUp, nativePostSourceBloom+level+1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	combineIndex := index + 2*passes + 1
	if err := r.writePostUniform(combineIndex, func(data []byte) { nativePostF32(data, 0, strength) }); err != nil {
		return err
	}
	return r.encodeNativePostCombine2(encoder, nativePostBlurCombine, sourceID, source.view, r.postBloom[0].view, destination, combineIndex)
}

// encodeNativePostCombine2 is encodePostCombine generalized over the
// pipeline name, since Blur's combine shares the two-texture bind layout but
// a different pipeline than bloom's combine.
func (r *Renderer) encodeNativePostCombine2(encoder *wgpu.CommandEncoder, pipelineName string, sourceID int, source, second, destination *wgpu.TextureView, uniformIndex int) error {
	pipeline := r.postPipelines[pipelineName]
	key := nativePostBindKey{pipeline: pipelineName, source: sourceID, bloom: nativePostSourceBloom, uniform: uniformIndex}
	bindGroup, err := r.cachedPostBindGroup(key, func() (*wgpu.BindGroup, error) {
		layout := pipeline.GetBindGroupLayout(0)
		defer layout.Release()
		return r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{
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

// nativeClampPostStrength mirrors platform/raster/postfx.go clamp01 for the
// one Blur field the encode loop must gate on before spending a pyramid pass.
func nativeClampPostStrength(value float64) float64 {
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
func (r *Renderer) cachedPostBindGroup(key nativePostBindKey, build func() (*wgpu.BindGroup, error)) (*wgpu.BindGroup, error) {
	if group, ok := r.postBindGroups[key]; ok {
		return group, nil
	}
	if r.postBindGroups == nil {
		r.postBindGroups = map[nativePostBindKey]*wgpu.BindGroup{}
	}
	group, err := build()
	if err != nil {
		return nil, err
	}
	r.postBindGroups[key] = group
	return group, nil
}

func (r *Renderer) postBindGroup(key nativePostBindKey, source *wgpu.TextureView) (*wgpu.BindGroup, error) {
	return r.cachedPostBindGroup(key, func() (*wgpu.BindGroup, error) {
		pipeline := r.postPipelines[key.pipeline]
		layout := pipeline.GetBindGroupLayout(0)
		defer layout.Release()
		entries := []wgpu.BindGroupEntry{{Binding: 0, TextureView: source}, {Binding: 1, Sampler: r.postSampler}}
		if key.uniform >= 0 {
			entries = append(entries, wgpu.BindGroupEntry{Binding: 2, Buffer: r.postUniforms[key.uniform], Size: wgpu.WholeSize})
		}
		return r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: entries})
	})
}

func (r *Renderer) encodePostPass(encoder *wgpu.CommandEncoder, pipelineName string, sourceID int, source, destination *wgpu.TextureView, uniformIndex int) error {
	pipeline := r.postPipelines[pipelineName]
	key := nativePostBindKey{pipeline: pipelineName, source: sourceID, bloom: -1, uniform: uniformIndex}
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
	pipeline := r.postPipelines[nativePostBloomCombine]
	key := nativePostBindKey{pipeline: nativePostBloomCombine, source: sourceID, bloom: nativePostSourceBloom, uniform: uniformIndex}
	bindGroup, err := r.cachedPostBindGroup(key, func() (*wgpu.BindGroup, error) {
		layout := pipeline.GetBindGroupLayout(0)
		defer layout.Release()
		return r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{
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

func (r *Renderer) encodeNativeBloom(encoder *wgpu.CommandEncoder, source nativePostTarget, sourceID int, destination *wgpu.TextureView, effect render.Bloom, index int) error {
	if effect.Intensity == 0 {
		return nil
	}
	passes := nativePostBloomPasses(effect)
	if err := r.ensurePostBloom(passes); err != nil {
		return err
	}
	if err := r.writePostUniform(index, func(dst []byte) { nativePostF32(dst, 0, effect.Threshold) }); err != nil {
		return err
	}
	if err := r.encodePostPass(encoder, nativePostBloomBright, sourceID, source.view, r.postBloom[0].view, index); err != nil {
		return err
	}
	for level := 1; level <= passes; level++ {
		src, dst := r.postBloom[level-1], r.postBloom[level]
		uniformIndex := index + level
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			nativePostF32(data, 0, float64(src.width))
			nativePostF32(data, 4, float64(src.height))
			nativePostF32(data, 8, float64(dst.width))
			nativePostF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, nativePostBloomDown, nativePostSourceBloom+level-1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	for level := passes - 1; level >= 0; level-- {
		src, dst := r.postBloom[level+1], r.postBloom[level]
		uniformIndex := index + passes + 1 + (passes - 1 - level)
		err := r.writePostUniform(uniformIndex, func(data []byte) {
			nativePostF32(data, 0, float64(src.width))
			nativePostF32(data, 4, float64(src.height))
			nativePostF32(data, 8, float64(dst.width))
			nativePostF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, nativePostBloomUp, nativePostSourceBloom+level+1, src.view, dst.view, uniformIndex); err != nil {
			return err
		}
	}
	combineIndex := index + 2*passes + 1
	if err := r.writePostUniform(combineIndex, func(data []byte) { nativePostF32(data, 0, effect.Intensity) }); err != nil {
		return err
	}
	return r.encodePostCombine(encoder, sourceID, source.view, r.postBloom[0].view, destination, combineIndex)
}

func (r *Renderer) encodePostFrame(runs []tess.StreamRun, surfaceView *wgpu.TextureView, frame runtime.Frame, chain render.PostChain, width, height int) error {
	if err := r.ensurePostTargets(width, height); err != nil {
		return err
	}
	encoder, err := r.device.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("create post command encoder: %w", err)
	}
	defer encoder.Release()
	if r.particles != nil {
		sims := make([]particleSim, 0)
		for _, run := range runs {
			if run.Kind != tess.DrawRunParticles {
				continue
			}
			if sim, ok := r.particles.prepare(run.Particles, frame.Camera, width, height); ok {
				sims = append(sims, sim)
			}
		}
		if err := r.particles.encode(encoder, sims, nil, nil); err != nil {
			return err
		}
	}
	attachment := wgpu.RenderPassColorAttachment{View: r.msaaView, ResolveTarget: r.postSceneA.view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpDiscard, ClearValue: clearColor(r.clear)}
	if r.msaaSamples <= 1 {
		attachment = wgpu.RenderPassColorAttachment{View: r.postSceneA.view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: clearColor(r.clear)}
	}
	pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{attachment}})
	last := -1
	for _, run := range runs {
		if !nativePostRunVisible(run, chain.BelowLayer, true) {
			continue
		}
		if err := r.drawRun(pass, run, frame.Camera, width, height, -1, false, &last); err != nil {
			pass.Release()
			return err
		}
	}
	if err := pass.TryEnd(); err != nil {
		pass.Release()
		return err
	}
	pass.Release()

	// Without MSAA and without runs above the chain, the last active effect
	// renders directly to the surface and the final blit pass disappears.
	lastActive := -1
	for i, effect := range chain.Effects {
		if nativePostEffectEncodes(effect) {
			lastActive = i
		}
	}
	directFinal := r.msaaSamples <= 1 && lastActive >= 0 &&
		!hasNativePostRuns(runs, chain.BelowLayer, false)

	source, destination := r.postSceneA, r.postSceneB
	sourceID, destID := nativePostSourceSceneA, nativePostSourceSceneB
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
			if err := r.encodeNativeBloom(encoder, source, sourceID, destView, bloom, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			sourceID, destID = destID, sourceID
			continue
		}
		if blur, ok := effect.(render.Blur); ok {
			if nativeClampPostStrength(blur.Strength) == 0 {
				continue
			}
			if err := r.encodeNativeBlur(encoder, source, sourceID, destView, blur, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			sourceID, destID = destID, sourceID
			continue
		}
		name, err := r.postParams(uniformIndex, effect, frame.Camera, chain.Time, width, height)
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
		blit := r.postPipelines[nativePostFinalBlit]
		key := nativePostBindKey{pipeline: nativePostFinalBlit, source: sourceID, bloom: -1, uniform: -1}
		bindGroup, err := r.postBindGroup(key, source.view)
		if err != nil {
			finalPass.Release()
			return err
		}
		finalPass.SetPipeline(blit)
		finalPass.SetBindGroup(0, bindGroup, nil)
		finalPass.Draw(3, 1, 0, 0)
		last = -1
		for _, run := range runs {
			if !nativePostRunVisible(run, chain.BelowLayer, false) {
				continue
			}
			if err := r.drawRun(finalPass, run, frame.Camera, width, height, -1, false, &last); err != nil {
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
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish post command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.device.queue.Submit(commandBuffer)
	return nil
}

// nativePostEffectEncodes mirrors the skip rules of the encode loop: a
// zero-intensity bloom and unknown effect types produce no pass.
func nativePostEffectEncodes(effect render.PostEffect) bool {
	switch effect := effect.(type) {
	case render.Bloom:
		return effect.Intensity != 0
	case render.Blur:
		return nativeClampPostStrength(effect.Strength) != 0
	case render.Vignette, render.ColorGrade, render.Pixelate,
		render.ChromaticAberration, render.Shockwave, render.LensRain,
		render.Fade, render.CircleWipe, render.Dissolve, render.FilmGrain,
		render.Scanlines, render.RadialBlur:
		return true
	default:
		return false
	}
}

func hasNativePostRuns(runs []tess.StreamRun, belowLayer int, below bool) bool {
	for _, run := range runs {
		if nativePostRunVisible(run, belowLayer, below) {
			return true
		}
	}
	return false
}

func nativePostRunVisible(run tess.StreamRun, belowLayer int, below bool) bool {
	if belowLayer == 0 {
		return below
	}
	if below {
		return run.Layer < belowLayer
	}
	return run.Layer >= belowLayer
}

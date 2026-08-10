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
)

type nativePostTarget struct {
	texture *wgpu.Texture
	view    *wgpu.TextureView
	width   int
	height  int
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

func (r *Renderer) writePostUniform(index int, write func([]byte)) (*wgpu.Buffer, error) {
	if err := r.ensurePostUniforms(index + 1); err != nil {
		return nil, err
	}
	data := make([]byte, 64)
	write(data)
	if err := r.device.queue.TryWriteBuffer(r.postUniforms[index], 0, data); err != nil {
		return nil, fmt.Errorf("write post uniform: %w", err)
	}
	return r.postUniforms[index], nil
}

func (r *Renderer) postParams(index int, effect render.PostEffect, camera render.Camera, time float64, width, height int) (string, *wgpu.Buffer, error) {
	switch effect := effect.(type) {
	case render.Vignette:
		buffer, err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Color.R)
			nativePostF32(dst, 4, effect.Color.G)
			nativePostF32(dst, 8, effect.Color.B)
			nativePostF32(dst, 12, effect.Color.A)
			nativePostF32(dst, 16, effect.Strength)
			nativePostF32(dst, 20, effect.Softness)
		})
		return nativePostVignette, buffer, err
	case render.ColorGrade:
		tint := effect.Tint
		if tint == (render.Color{}) {
			tint = render.RGBA(1, 1, 1, 1)
		}
		buffer, err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Brightness)
			nativePostF32(dst, 4, effect.Contrast)
			nativePostF32(dst, 8, effect.Saturation)
			nativePostF32(dst, 16, tint.R)
			nativePostF32(dst, 20, tint.G)
			nativePostF32(dst, 24, tint.B)
		})
		return nativePostColorGrade, buffer, err
	case render.Pixelate:
		buffer, err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Size)
			nativePostF32(dst, 4, float64(width))
			nativePostF32(dst, 8, float64(height))
		})
		return nativePostPixelate, buffer, err
	case render.ChromaticAberration:
		buffer, err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Strength)
			nativePostF32(dst, 4, float64(width))
			nativePostF32(dst, 8, float64(height))
		})
		return nativePostChromatic, buffer, err
	case render.Shockwave:
		center := camera.WorldToScreen(effect.Center)
		buffer, err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, center.X)
			nativePostF32(dst, 4, center.Y)
			nativePostF32(dst, 8, math.Abs(camera.WorldScalarToScreen(effect.Radius)))
			nativePostF32(dst, 12, math.Abs(camera.WorldScalarToScreen(effect.Width)))
			nativePostF32(dst, 16, effect.Strength)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
		})
		return nativePostShockwave, buffer, err
	case render.LensRain:
		buffer, err := r.writePostUniform(index, func(dst []byte) {
			nativePostF32(dst, 0, effect.Density)
			nativePostF32(dst, 4, effect.Size)
			nativePostF32(dst, 8, effect.Speed)
			nativePostF32(dst, 12, effect.Refraction)
			nativePostF32(dst, 16, time)
			nativePostF32(dst, 20, float64(width))
			nativePostF32(dst, 24, float64(height))
			nativePostU32(dst, 28, uint32(effect.Seed))
		})
		return nativePostLensRain, buffer, err
	default:
		return "", nil, nil
	}
}

func (r *Renderer) postBindGroup(pipeline *wgpu.RenderPipeline, source *wgpu.TextureView, uniform *wgpu.Buffer) (*wgpu.BindGroup, error) {
	layout := pipeline.GetBindGroupLayout(0)
	defer layout.Release()
	entries := []wgpu.BindGroupEntry{{Binding: 0, TextureView: source}, {Binding: 1, Sampler: r.postSampler}}
	if uniform != nil {
		entries = append(entries, wgpu.BindGroupEntry{Binding: 2, Buffer: uniform, Size: wgpu.WholeSize})
	}
	return r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: entries})
}

func (r *Renderer) encodePostPass(encoder *wgpu.CommandEncoder, source, destination *wgpu.TextureView, pipeline *wgpu.RenderPipeline, uniform *wgpu.Buffer) error {
	pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: destination, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{A: 1},
	}}})
	bindGroup, err := r.postBindGroup(pipeline, source, uniform)
	if err != nil {
		pass.Release()
		return err
	}
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, bindGroup, nil)
	pass.Draw(3, 1, 0, 0)
	bindGroup.Release()
	if err := pass.TryEnd(); err != nil {
		pass.Release()
		return err
	}
	pass.Release()
	return nil
}

func (r *Renderer) encodePostCombine(encoder *wgpu.CommandEncoder, source, bloom, destination *wgpu.TextureView, uniform *wgpu.Buffer) error {
	pipeline := r.postPipelines[nativePostBloomCombine]
	layout := pipeline.GetBindGroupLayout(0)
	defer layout.Release()
	bindGroup, err := r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{
		{Binding: 0, TextureView: source}, {Binding: 1, TextureView: bloom}, {Binding: 2, Sampler: r.postSampler},
		{Binding: 3, Buffer: uniform, Size: wgpu.WholeSize},
	}})
	if err != nil {
		return err
	}
	pass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{
		View: destination, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{A: 1},
	}}})
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, bindGroup, nil)
	pass.Draw(3, 1, 0, 0)
	bindGroup.Release()
	if err := pass.TryEnd(); err != nil {
		pass.Release()
		return err
	}
	pass.Release()
	return nil
}

func (r *Renderer) encodeNativeBloom(encoder *wgpu.CommandEncoder, source, destination nativePostTarget, effect render.Bloom, index int) error {
	if effect.Intensity == 0 {
		return nil
	}
	passes := nativePostBloomPasses(effect)
	if err := r.ensurePostBloom(passes); err != nil {
		return err
	}
	bright, err := r.writePostUniform(index, func(dst []byte) { nativePostF32(dst, 0, effect.Threshold) })
	if err != nil {
		return err
	}
	if err := r.encodePostPass(encoder, source.view, r.postBloom[0].view, r.postPipelines[nativePostBloomBright], bright); err != nil {
		return err
	}
	for level := 1; level <= passes; level++ {
		src, dst := r.postBloom[level-1], r.postBloom[level]
		uniform, err := r.writePostUniform(index+level, func(data []byte) {
			nativePostF32(data, 0, float64(src.width))
			nativePostF32(data, 4, float64(src.height))
			nativePostF32(data, 8, float64(dst.width))
			nativePostF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, src.view, dst.view, r.postPipelines[nativePostBloomDown], uniform); err != nil {
			return err
		}
	}
	for level := passes - 1; level >= 0; level-- {
		src, dst := r.postBloom[level+1], r.postBloom[level]
		uniform, err := r.writePostUniform(index+passes+1+(passes-1-level), func(data []byte) {
			nativePostF32(data, 0, float64(src.width))
			nativePostF32(data, 4, float64(src.height))
			nativePostF32(data, 8, float64(dst.width))
			nativePostF32(data, 12, float64(dst.height))
		})
		if err != nil {
			return err
		}
		if err := r.encodePostPass(encoder, src.view, dst.view, r.postPipelines[nativePostBloomUp], uniform); err != nil {
			return err
		}
	}
	combine, err := r.writePostUniform(index+2*passes+1, func(data []byte) { nativePostF32(data, 0, effect.Intensity) })
	if err != nil {
		return err
	}
	return r.encodePostCombine(encoder, source.view, r.postBloom[0].view, destination.view, combine)
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

	source, destination := r.postSceneA, r.postSceneB
	uniformIndex := 0
	for _, effect := range chain.Effects {
		if bloom, ok := effect.(render.Bloom); ok {
			if bloom.Intensity == 0 {
				continue
			}
			if err := r.encodeNativeBloom(encoder, source, destination, bloom, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			continue
		}
		name, uniform, err := r.postParams(uniformIndex, effect, frame.Camera, chain.Time, width, height)
		if err != nil {
			return err
		}
		uniformIndex++
		if name == "" {
			continue
		}
		if err := r.encodePostPass(encoder, source.view, destination.view, r.postPipelines[name], uniform); err != nil {
			return err
		}
		source, destination = destination, source
	}
	final := wgpu.RenderPassColorAttachment{View: r.msaaView, ResolveTarget: surfaceView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpDiscard, ClearValue: clearColor(r.clear)}
	if r.msaaSamples <= 1 {
		final = wgpu.RenderPassColorAttachment{View: surfaceView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: clearColor(r.clear)}
	}
	finalPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{final}})
	blit := r.postPipelines[nativePostFinalBlit]
	bindGroup, err := r.postBindGroup(blit, source.view, nil)
	if err != nil {
		finalPass.Release()
		return err
	}
	finalPass.SetPipeline(blit)
	finalPass.SetBindGroup(0, bindGroup, nil)
	finalPass.Draw(3, 1, 0, 0)
	bindGroup.Release()
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
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish post command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.device.queue.Submit(commandBuffer)
	return nil
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

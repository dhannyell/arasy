//go:build js && wasm

package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"

	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/platform/wasm/gpu"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

const (
	postPipelineBlit         = "blit"
	postPipelineFinalBlit    = "final-blit"
	postPipelineVignette     = "vignette"
	postPipelineColorGrade   = "color-grade"
	postPipelinePixelate     = "pixelate"
	postPipelineChromatic    = "chromatic"
	postPipelineBloomBright  = "bloom-bright"
	postPipelineBloomDown    = "bloom-down"
	postPipelineBloomUp      = "bloom-up"
	postPipelineBloomCombine = "bloom-combine"
	postPipelineShockwave    = "shockwave"
	postPipelineLensRain     = "lens-rain"
)

// webgpuPostTarget is a single-sample texture used by the post-process
// ping-pong. Its format is the configured canvas format so it can be resolved
// from an MSAA scene pass and sampled by the fullscreen shaders.
type webgpuPostTarget struct {
	texture js.Value
	view    js.Value
	width   int
	height  int
}

func (r *webgpuRenderer) initPostResources() {
	r.postSampler = r.device.Call("createSampler", map[string]any{
		"magFilter": "linear",
		"minFilter": "linear",
	})
	r.postPipelines = map[string]js.Value{
		postPipelineBlit:         r.createPostPipeline(postPipelineBlit, shaders.PostBlitWGSL, "fs_main", 1),
		postPipelineFinalBlit:    r.createPostPipeline(postPipelineFinalBlit, shaders.PostBlitWGSL, "fs_main", r.msaaSamples),
		postPipelineVignette:     r.createPostPipeline(postPipelineVignette, shaders.PostVignetteWGSL, "fs_main", 1),
		postPipelineColorGrade:   r.createPostPipeline(postPipelineColorGrade, shaders.PostColorGradeWGSL, "fs_main", 1),
		postPipelinePixelate:     r.createPostPipeline(postPipelinePixelate, shaders.PostPixelateWGSL, "fs_main", 1),
		postPipelineChromatic:    r.createPostPipeline(postPipelineChromatic, shaders.PostChromaticWGSL, "fs_main", 1),
		postPipelineBloomBright:  r.createPostPipeline(postPipelineBloomBright, shaders.PostBloomWGSL, "fs_bright", 1),
		postPipelineBloomDown:    r.createPostPipeline(postPipelineBloomDown, shaders.PostBloomWGSL, "fs_down", 1),
		postPipelineBloomUp:      r.createPostPipeline(postPipelineBloomUp, shaders.PostBloomWGSL, "fs_up", 1),
		postPipelineBloomCombine: r.createPostPipeline(postPipelineBloomCombine, shaders.PostBloomCombineWGSL, "fs_main", 1),
		postPipelineShockwave:    r.createPostPipeline(postPipelineShockwave, shaders.PostShockwaveWGSL, "fs_main", 1),
		postPipelineLensRain:     r.createPostPipeline(postPipelineLensRain, shaders.PostLensRainWGSL, "fs_main", 1),
	}
}

func (r *webgpuRenderer) createPostPipeline(label, source, fragmentEntry string, samples int) js.Value {
	module := r.device.Call("createShaderModule", map[string]any{
		"label": label + " shader",
		"code":  source,
	})
	return r.device.Call("createRenderPipeline", map[string]any{
		"label":  label,
		"layout": "auto",
		"vertex": map[string]any{"module": module, "entryPoint": "vs_main"},
		"fragment": map[string]any{
			"module":     module,
			"entryPoint": fragmentEntry,
			"targets":    []any{map[string]any{"format": r.format}},
		},
		"primitive":   map[string]any{"topology": "triangle-list"},
		"multisample": map[string]any{"count": samples},
	})
}

func (r *webgpuRenderer) ensurePostUniforms(count int) {
	for len(r.postUniforms) < count {
		r.postUniforms = append(r.postUniforms,
			jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(64, gpu.BufferUsageUniform|gpu.BufferUsageCopyDst))))
	}
}

func (r *webgpuRenderer) ensurePostTargets(width, height int) error {
	if r.postWidth == width && r.postHeight == height &&
		!r.postSceneA.texture.IsUndefined() && !r.postSceneA.texture.IsNull() {
		return nil
	}
	r.releasePostTarget(&r.postSceneA)
	r.releasePostTarget(&r.postSceneB)
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = nil
	create := func() (webgpuPostTarget, error) {
		texture := r.device.Call("createTexture", map[string]any{
			"size":   []any{width, height},
			"format": r.format,
			"usage":  gpu.TextureUsageRenderAttachment | gpu.TextureUsageTextureBinding,
		})
		if texture.IsUndefined() || texture.IsNull() {
			return webgpuPostTarget{}, fmt.Errorf("create WebGPU post target: format %s is not sampleable", r.format.String())
		}
		view := texture.Call("createView")
		return webgpuPostTarget{texture: texture, view: view, width: width, height: height}, nil
	}
	var err error
	if r.postSceneA, err = create(); err != nil {
		return err
	}
	if r.postSceneB, err = create(); err != nil {
		r.releasePostTarget(&r.postSceneA)
		return err
	}
	r.postWidth, r.postHeight = width, height
	return nil
}

func (r *webgpuRenderer) ensureBloomTargets(passes int) error {
	if passes < 1 {
		passes = 1
	}
	if len(r.postBloom) == passes+1 && r.postBloom[0].width == r.postWidth && r.postBloom[0].height == r.postHeight {
		return nil
	}
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = make([]webgpuPostTarget, 0, passes+1)
	width, height := r.postWidth, r.postHeight
	for level := 0; level <= passes; level++ {
		texture := r.device.Call("createTexture", map[string]any{
			"size":   []any{width, height},
			"format": r.format,
			"usage":  gpu.TextureUsageRenderAttachment | gpu.TextureUsageTextureBinding,
		})
		if texture.IsUndefined() || texture.IsNull() {
			for index := range r.postBloom {
				r.releasePostTarget(&r.postBloom[index])
			}
			r.postBloom = nil
			return fmt.Errorf("create WebGPU bloom target: format %s is not sampleable", r.format.String())
		}
		r.postBloom = append(r.postBloom, webgpuPostTarget{
			texture: texture,
			view:    texture.Call("createView"),
			width:   width,
			height:  height,
		})
		width, height = max(1, (width+1)/2), max(1, (height+1)/2)
	}
	return nil
}

func (r *webgpuRenderer) releasePostTarget(target *webgpuPostTarget) {
	if target.view.IsUndefined() == false && !target.view.IsNull() {
		target.view = js.Undefined()
	}
	if !target.texture.IsUndefined() && !target.texture.IsNull() {
		target.texture.Call("destroy")
	}
	target.texture = js.Undefined()
	target.view = js.Undefined()
	target.width, target.height = 0, 0
}

func lastWebGPUPostChain(commands []render.Command) (render.PostChain, bool) {
	var chain render.PostChain
	found := false
	for _, command := range commands {
		if candidate, ok := command.(render.PostChain); ok {
			chain, found = candidate, true
		}
	}
	return chain, found
}

func postUniformBuffer(r *webgpuRenderer, index int, values func([]byte)) js.Value {
	r.ensurePostUniforms(index + 1)
	data := make([]byte, 64)
	values(data)
	r.queue.Call("writeBuffer", r.postUniforms[index], 0, bytesToJS(data))
	return r.postUniforms[index]
}

func putPostF32(dst []byte, index int, value float64) {
	binary.LittleEndian.PutUint32(dst[index:], math.Float32bits(float32(value)))
}

func putPostU32(dst []byte, index int, value uint32) {
	binary.LittleEndian.PutUint32(dst[index:], value)
}

func (r *webgpuRenderer) postParams(index int, effect render.PostEffect, camera render.Camera, time float64, width, height int) (string, js.Value) {
	switch effect := effect.(type) {
	case render.Vignette:
		return postPipelineVignette, postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Color.R)
			putPostF32(dst, 4, effect.Color.G)
			putPostF32(dst, 8, effect.Color.B)
			putPostF32(dst, 12, effect.Color.A)
			putPostF32(dst, 16, effect.Strength)
			putPostF32(dst, 20, effect.Softness)
		})
	case render.ColorGrade:
		tint := effect.Tint
		if tint == (render.Color{}) {
			tint = render.RGBA(1, 1, 1, 1)
		}
		return postPipelineColorGrade, postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Brightness)
			putPostF32(dst, 4, effect.Contrast)
			putPostF32(dst, 8, effect.Saturation)
			putPostF32(dst, 16, tint.R)
			putPostF32(dst, 20, tint.G)
			putPostF32(dst, 24, tint.B)
		})
	case render.Pixelate:
		return postPipelinePixelate, postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Size)
			putPostF32(dst, 4, float64(width))
			putPostF32(dst, 8, float64(height))
		})
	case render.ChromaticAberration:
		return postPipelineChromatic, postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Strength)
			putPostF32(dst, 4, float64(width))
			putPostF32(dst, 8, float64(height))
		})
	case render.Shockwave:
		center := camera.WorldToScreen(effect.Center)
		return postPipelineShockwave, postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, center.X)
			putPostF32(dst, 4, center.Y)
			putPostF32(dst, 8, math.Abs(camera.WorldScalarToScreen(effect.Radius)))
			putPostF32(dst, 12, math.Abs(camera.WorldScalarToScreen(effect.Width)))
			putPostF32(dst, 16, effect.Strength)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
		})
	case render.LensRain:
		return postPipelineLensRain, postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Density)
			putPostF32(dst, 4, effect.Size)
			putPostF32(dst, 8, effect.Speed)
			putPostF32(dst, 12, effect.Refraction)
			putPostF32(dst, 16, time)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
			putPostU32(dst, 28, uint32(effect.Seed))
		})
	default:
		return "", js.Undefined()
	}
}

func (r *webgpuRenderer) postBindGroup(pipeline, source js.Value, uniform js.Value) js.Value {
	entries := []any{
		map[string]any{"binding": 0, "resource": source},
		map[string]any{"binding": 1, "resource": r.postSampler},
	}
	if !uniform.IsUndefined() && !uniform.IsNull() {
		entries = append(entries, map[string]any{"binding": 2, "resource": map[string]any{"buffer": uniform}})
	}
	return r.device.Call("createBindGroup", map[string]any{
		"layout":  pipeline.Call("getBindGroupLayout", 0),
		"entries": entries,
	})
}

func (r *webgpuRenderer) encodePostPass(encoder, source, destination js.Value, pipeline, uniform js.Value) {
	pass := encoder.Call("beginRenderPass", map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":       destination,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
		}},
	})
	r.drawPostFullscreen(pass, source, pipeline, uniform)
	pass.Call("end")
}

func (r *webgpuRenderer) encodePostCombine(encoder, source, bloom, destination, uniform js.Value) {
	pass := encoder.Call("beginRenderPass", map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":       destination,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
		}},
	})
	pipeline := r.postPipelines[postPipelineBloomCombine]
	bindGroup := r.device.Call("createBindGroup", map[string]any{
		"layout": pipeline.Call("getBindGroupLayout", 0),
		"entries": []any{
			map[string]any{"binding": 0, "resource": source},
			map[string]any{"binding": 1, "resource": bloom},
			map[string]any{"binding": 2, "resource": r.postSampler},
			map[string]any{"binding": 3, "resource": map[string]any{"buffer": uniform}},
		},
	})
	pass.Call("setPipeline", pipeline)
	pass.Call("setBindGroup", 0, bindGroup)
	pass.Call("draw", 3, 1, 0, 0)
	pass.Call("end")
}

func postBloomPasses(effect render.Bloom) int {
	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	return max(1, min(5, passes))
}

func (r *webgpuRenderer) encodeBloom(encoder js.Value, source, destination webgpuPostTarget, effect render.Bloom, index int) error {
	if effect.Intensity == 0 {
		return nil
	}
	passes := postBloomPasses(effect)
	if err := r.ensureBloomTargets(passes); err != nil {
		return err
	}
	brightUniform := postUniformBuffer(r, index, func(dst []byte) {
		putPostF32(dst, 0, effect.Threshold)
	})
	r.encodePostPass(encoder, source.view, r.postBloom[0].view, r.postPipelines[postPipelineBloomBright], brightUniform)
	for level := 1; level <= passes; level++ {
		src := r.postBloom[level-1]
		dst := r.postBloom[level]
		uniform := postUniformBuffer(r, index+level, func(data []byte) {
			putPostF32(data, 0, float64(src.width))
			putPostF32(data, 4, float64(src.height))
			putPostF32(data, 8, float64(dst.width))
			putPostF32(data, 12, float64(dst.height))
		})
		r.encodePostPass(encoder, src.view, dst.view, r.postPipelines[postPipelineBloomDown], uniform)
	}
	for level := passes - 1; level >= 0; level-- {
		src := r.postBloom[level+1]
		dst := r.postBloom[level]
		uniform := postUniformBuffer(r, index+passes+1+(passes-1-level), func(data []byte) {
			putPostF32(data, 0, float64(src.width))
			putPostF32(data, 4, float64(src.height))
			putPostF32(data, 8, float64(dst.width))
			putPostF32(data, 12, float64(dst.height))
		})
		r.encodePostPass(encoder, src.view, dst.view, r.postPipelines[postPipelineBloomUp], uniform)
	}
	combineUniform := postUniformBuffer(r, index+2*passes+1, func(data []byte) {
		putPostF32(data, 0, effect.Intensity)
	})
	r.encodePostCombine(encoder, source.view, r.postBloom[0].view, destination.view, combineUniform)
	return nil
}

func (r *webgpuRenderer) drawPostFullscreen(pass, source, pipeline, uniform js.Value) {
	pass.Call("setPipeline", pipeline)
	pass.Call("setBindGroup", 0, r.postBindGroup(pipeline, source, uniform))
	pass.Call("draw", 3, 1, 0, 0)
}

func (r *webgpuRenderer) renderPostFrame(
	frame runtime.Frame,
	runs []gpuDrawRun,
	chain render.PostChain,
	w, h int,
) error {
	if err := r.ensurePostTargets(w, h); err != nil {
		return err
	}
	encoder := r.device.Call("createCommandEncoder")
	textureView := r.context.Call("getCurrentTexture").Call("createView")
	if r.particles != nil {
		particles := make([]render.DrawParticles, 0)
		for _, run := range runs {
			if run.kind == tess.DrawRunParticles {
				particles = append(particles, run.particles)
			}
		}
		r.particles.encodeSims(encoder, particles, frame.Camera, w, h)
	}

	attachment := map[string]any{
		"view":          r.msaaView,
		"resolveTarget": r.postSceneA.view,
		"loadOp":        "clear",
		"storeOp":       "discard",
		"clearValue":    clearValue(r.clear),
	}
	if r.msaaSamples <= 1 {
		attachment = map[string]any{
			"view":       r.postSceneA.view,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": clearValue(r.clear),
		}
	}
	pass := encoder.Call("beginRenderPass", map[string]any{"colorAttachments": []any{attachment}})
	r.drawPostRuns(pass, runs, frame, chain.BelowLayer, true, w, h)
	pass.Call("end")

	source, destination := r.postSceneA, r.postSceneB
	uniformIndex := 0
	for _, effect := range chain.Effects {
		if bloom, ok := effect.(render.Bloom); ok {
			if bloom.Intensity == 0 {
				continue
			}
			if err := r.encodeBloom(encoder, source, destination, bloom, uniformIndex); err != nil {
				return err
			}
			uniformIndex += 16
			source, destination = destination, source
			continue
		}
		pipelineName, uniform := r.postParams(uniformIndex, effect, frame.Camera, chain.Time, w, h)
		uniformIndex++
		if pipelineName == "" {
			continue
		}
		r.encodePostPass(encoder, source.view, destination.view, r.postPipelines[pipelineName], uniform)
		source, destination = destination, source
	}

	finalAttachment := map[string]any{
		"view":          r.msaaView,
		"resolveTarget": textureView,
		"loadOp":        "clear",
		"storeOp":       "discard",
		"clearValue":    clearValue(r.clear),
	}
	if r.msaaSamples <= 1 {
		finalAttachment = map[string]any{
			"view":       textureView,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": clearValue(r.clear),
		}
	}
	finalPass := encoder.Call("beginRenderPass", map[string]any{"colorAttachments": []any{finalAttachment}})
	r.drawPostFullscreen(finalPass, source.view, r.postPipelines[postPipelineFinalBlit], js.Undefined())
	r.drawPostRuns(finalPass, runs, frame, chain.BelowLayer, false, w, h)
	finalPass.Call("end")
	commandBuffer := encoder.Call("finish")
	r.queue.Call("submit", []any{commandBuffer})
	if r.particles != nil {
		r.particles.endFrame()
	}
	r.signalReadyAfterPresent()
	return nil
}

func postRunVisible(run gpuDrawRun, below bool, belowLayer int) bool {
	if belowLayer == 0 {
		return below
	}
	if below {
		return run.layer < belowLayer
	}
	return run.layer >= belowLayer
}

func (r *webgpuRenderer) drawPostRuns(
	pass js.Value,
	runs []gpuDrawRun,
	frame runtime.Frame,
	belowLayer int,
	below bool,
	w, h int,
) {
	for _, run := range runs {
		if !postRunVisible(run, below, belowLayer) {
			continue
		}
		switch run.kind {
		case tess.DrawRunSolid:
			pass.Call("setPipeline", pickJS(run.blend, r.solidAddPipeline, r.solidPipeline))
			pass.Call("setVertexBuffer", 0, r.solidBuffer)
			pass.Call("draw", run.count, 1, run.offset, 0)
		case tess.DrawRunTextured:
			texture, ok := r.textureFor(run.texture)
			if !ok {
				continue
			}
			pass.Call("setPipeline", pickJS(run.blend, r.texturedAddPipeline, r.texturedPipeline))
			pass.Call("setVertexBuffer", 0, r.texBuffer)
			pass.Call("setBindGroup", 0, texture.bindGroup)
			pass.Call("draw", run.count, 1, run.offset, 0)
		case tess.DrawRunSprites:
			if r.spriteArrayEnabled {
				pass.Call("setPipeline", pickJS(run.blend, r.spriteArrayAddPipeline, r.spriteArrayPipeline))
				pass.Call("setBindGroup", 0, r.spriteArrayBindGroup)
			} else {
				texture, ok := r.textureFor(run.texture)
				if !ok || texture.spriteBindGroup.IsUndefined() || texture.spriteBindGroup.IsNull() {
					continue
				}
				pass.Call("setPipeline", pickJS(run.blend, r.spriteAddPipeline, r.spritePipeline))
				pass.Call("setBindGroup", 0, texture.spriteBindGroup)
			}
			pass.Call("draw", run.count*6, 1, run.offset*6, 0)
		case tess.DrawRunText:
			atlas, ok := r.textAtlasFor(run.atlasKey)
			if !ok {
				continue
			}
			pass.Call("setPipeline", pickJS(run.blend, r.textGlyphAddPipeline, r.textGlyphPipeline))
			pass.Call("setBindGroup", 0, atlas.glyphBindGroup)
			pass.Call("draw", run.count*6, 1, run.offset*6, 0)
		case tess.DrawRunParticles:
			if r.particles != nil {
				r.particles.draw(pass, run.particles, frame.Camera, w, h)
			}
		case tess.DrawRunSpriteBatch:
			batch := run.batch
			if batch == nil {
				continue
			}
			cmd := run.spriteBatch
			if !applyClipScissor(pass, cmd.Clip, frame.Camera, 1, w, h) {
				continue
			}
			for i, rg := range batch.ranges {
				if batch.animated {
					if r.spriteArrayEnabled && !rg.magenta {
						pass.Call("setPipeline", r.spriteBatchAnimArrayPipeline)
					} else {
						pass.Call("setPipeline", r.spriteBatchAnimPipeline)
					}
					pass.Call("setBindGroup", 0, batch.bindGroups[i], []any{run.batchSlot})
				} else {
					if r.spriteArrayEnabled && !rg.magenta {
						pass.Call("setPipeline", r.spriteArrayPipeline)
					} else {
						pass.Call("setPipeline", r.spritePipeline)
					}
					pass.Call("setBindGroup", 0, batch.bindGroups[i])
				}
				pass.Call("draw", rg.count*6, 1, rg.start*6, 0)
			}
			if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
				pass.Call("setScissorRect", 0, 0, w, h)
			}
		case tess.DrawRunTextBatch:
			batch := run.text
			if batch == nil {
				continue
			}
			cmd := run.textBatch
			if !applyClipScissor(pass, cmd.Clip, frame.Camera, 1, w, h) {
				continue
			}
			pass.Call("setPipeline", r.textGlyphPipeline)
			for _, rg := range batch.ranges {
				if rg.bindGroup.IsUndefined() || rg.bindGroup.IsNull() {
					continue
				}
				pass.Call("setBindGroup", 0, rg.bindGroup)
				pass.Call("draw", rg.count*6, 1, rg.start*6, 0)
			}
			if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
				pass.Call("setScissorRect", 0, 0, w, h)
			}
		}
	}
}

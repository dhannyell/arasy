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
	postPipelineBlurDown     = "blur-down"
	postPipelineBlurUp       = "blur-up"
	postPipelineBlurCombine  = "blur-combine"
	postPipelineFade         = "fade"
	postPipelineCircleWipe   = "circle-wipe"
	postPipelineDissolve     = "dissolve"
	postPipelineGrain        = "grain"
	postPipelineScanlines    = "scanlines"
	postPipelineRadialBlur   = "radial-blur"
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
		// Blur reuses the bloom pyramid's fs_down/fs_up entry points (same
		// taps and weights) and only owns the final sharp/blurred mix.
		postPipelineBlurDown:    r.createPostPipeline(postPipelineBlurDown, shaders.PostBloomWGSL, "fs_down", 1),
		postPipelineBlurUp:      r.createPostPipeline(postPipelineBlurUp, shaders.PostBloomWGSL, "fs_up", 1),
		postPipelineBlurCombine: r.createPostPipeline(postPipelineBlurCombine, shaders.PostBlurWGSL, "fs_main", 1),
		postPipelineFade:        r.createPostPipeline(postPipelineFade, shaders.PostFadeWGSL, "fs_main", 1),
		postPipelineCircleWipe:  r.createPostPipeline(postPipelineCircleWipe, shaders.PostCircleWipeWGSL, "fs_main", 1),
		postPipelineDissolve:    r.createPostPipeline(postPipelineDissolve, shaders.PostDissolveWGSL, "fs_main", 1),
		postPipelineGrain:       r.createPostPipeline(postPipelineGrain, shaders.PostGrainWGSL, "fs_main", 1),
		postPipelineScanlines:   r.createPostPipeline(postPipelineScanlines, shaders.PostScanlinesWGSL, "fs_main", 1),
		postPipelineRadialBlur:  r.createPostPipeline(postPipelineRadialBlur, shaders.PostRadialBlurWGSL, "fs_main", 1),
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
	clear(r.postBindGroups)
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

func (r *webgpuRenderer) disablePost(err error) {
	if r.postDisabled {
		return
	}
	r.postDisabled = true
	r.log.Warn("webgpu post-processing disabled; drawing without chain", "err", err)
	r.releasePostTarget(&r.postSceneA)
	r.releasePostTarget(&r.postSceneB)
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = nil
	clear(r.postBindGroups)
}

// postUniformBuffer writes one effect's parameters into the pooled uniform
// buffer at index. The scratch is cleared first so unwritten fields keep the
// deterministic zero the per-call allocation used to give them.
func postUniformBuffer(r *webgpuRenderer, index int, values func([]byte)) {
	r.ensurePostUniforms(index + 1)
	data := r.postUniformScratch[:]
	clear(data)
	values(data)
	if r.postUniformJS.IsUndefined() {
		r.postUniformJS = js.Global().Get("Uint8Array").New(len(data))
	}
	js.CopyBytesToJS(r.postUniformJS, data)
	r.queue.Call("writeBuffer", r.postUniforms[index], 0, r.postUniformJS)
}

func putPostF32(dst []byte, index int, value float64) {
	binary.LittleEndian.PutUint32(dst[index:], math.Float32bits(float32(value)))
}

func putPostU32(dst []byte, index int, value uint32) {
	binary.LittleEndian.PutUint32(dst[index:], value)
}

// postParams writes the effect's uniform at the given slot and returns the
// pipeline that consumes it, or "" for an effect this backend does not encode.
func (r *webgpuRenderer) postParams(index int, effect render.PostEffect, camera render.Camera, time float64, width, height int) string {
	switch effect := effect.(type) {
	case render.Vignette:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Color.R)
			putPostF32(dst, 4, effect.Color.G)
			putPostF32(dst, 8, effect.Color.B)
			putPostF32(dst, 12, effect.Color.A)
			putPostF32(dst, 16, effect.Strength)
			putPostF32(dst, 20, effect.Softness)
		})
		return postPipelineVignette
	case render.ColorGrade:
		tint := effect.Tint
		if tint == (render.Color{}) {
			tint = render.RGBA(1, 1, 1, 1)
		}
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Brightness)
			putPostF32(dst, 4, effect.Contrast)
			putPostF32(dst, 8, effect.Saturation)
			putPostF32(dst, 16, tint.R)
			putPostF32(dst, 20, tint.G)
			putPostF32(dst, 24, tint.B)
		})
		return postPipelineColorGrade
	case render.Pixelate:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Size)
			putPostF32(dst, 4, float64(width))
			putPostF32(dst, 8, float64(height))
		})
		return postPipelinePixelate
	case render.ChromaticAberration:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Strength)
			putPostF32(dst, 4, float64(width))
			putPostF32(dst, 8, float64(height))
		})
		return postPipelineChromatic
	case render.Shockwave:
		center := camera.WorldToScreen(effect.Center)
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, center.X)
			putPostF32(dst, 4, center.Y)
			putPostF32(dst, 8, math.Abs(camera.WorldScalarToScreen(effect.Radius)))
			putPostF32(dst, 12, math.Abs(camera.WorldScalarToScreen(effect.Width)))
			putPostF32(dst, 16, effect.Strength)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
		})
		return postPipelineShockwave
	case render.LensRain:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Density)
			putPostF32(dst, 4, effect.Size)
			putPostF32(dst, 8, effect.Speed)
			putPostF32(dst, 12, effect.Refraction)
			putPostF32(dst, 16, time)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
			putPostU32(dst, 28, uint32(effect.Seed))
		})
		return postPipelineLensRain
	case render.Fade:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Color.R)
			putPostF32(dst, 4, effect.Color.G)
			putPostF32(dst, 8, effect.Color.B)
			putPostF32(dst, 12, effect.Color.A)
			putPostF32(dst, 16, effect.Amount)
		})
		return postPipelineFade
	case render.CircleWipe:
		center := camera.WorldToScreen(effect.Center)
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Color.R)
			putPostF32(dst, 4, effect.Color.G)
			putPostF32(dst, 8, effect.Color.B)
			putPostF32(dst, 12, effect.Color.A)
			putPostF32(dst, 16, center.X)
			putPostF32(dst, 20, center.Y)
			putPostF32(dst, 24, effect.Progress)
			putPostF32(dst, 28, effect.Softness)
			putPostF32(dst, 32, float64(width))
			putPostF32(dst, 36, float64(height))
		})
		return postPipelineCircleWipe
	case render.Dissolve:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Color.R)
			putPostF32(dst, 4, effect.Color.G)
			putPostF32(dst, 8, effect.Color.B)
			putPostF32(dst, 12, effect.Color.A)
			putPostF32(dst, 16, effect.Amount)
			putPostF32(dst, 20, effect.CellSize)
			putPostF32(dst, 36, float64(width))
			putPostF32(dst, 40, float64(height))
			putPostU32(dst, 44, uint32(effect.Seed))
		})
		return postPipelineDissolve
	case render.FilmGrain:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Intensity)
			putPostF32(dst, 4, effect.Size)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
			putPostU32(dst, 28, uint32(effect.Seed)+uint32(time*60))
		})
		return postPipelineGrain
	case render.Scanlines:
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, effect.Intensity)
			putPostF32(dst, 4, effect.Thickness)
			putPostF32(dst, 8, effect.MaskStrength)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
		})
		return postPipelineScanlines
	case render.RadialBlur:
		center := camera.WorldToScreen(effect.Center)
		postUniformBuffer(r, index, func(dst []byte) {
			putPostF32(dst, 0, center.X)
			putPostF32(dst, 4, center.Y)
			putPostF32(dst, 8, effect.Strength)
			putPostF32(dst, 20, float64(width))
			putPostF32(dst, 24, float64(height))
		})
		return postPipelineRadialBlur
	default:
		return ""
	}
}

func postBlurPasses(effect render.Blur) int {
	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	return max(1, min(5, passes))
}

// encodeBlur mirrors encodeBloom's pyramid but seeds level zero directly from
// the scene (no bright pass) and combines with a sharp/blurred mix instead of
// an additive combine.
func (r *webgpuRenderer) encodeBlur(encoder js.Value, source webgpuPostTarget, sourceID int, destination js.Value, effect render.Blur, index int) error {
	strength := effect.Strength
	if strength <= 0 {
		return nil
	}
	passes := postBlurPasses(effect)
	if err := r.ensureBloomTargets(passes); err != nil {
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
		postUniformBuffer(r, uniformIndex, func(data []byte) {
			putPostF32(data, 0, float64(srcWidth))
			putPostF32(data, 4, float64(srcHeight))
			putPostF32(data, 8, float64(dst.width))
			putPostF32(data, 12, float64(dst.height))
		})
		r.encodePostPass(encoder, postPipelineBlurDown, srcID, srcView, dst.view, uniformIndex)
		srcView, srcID = dst.view, postSourceBloom+level
		srcWidth, srcHeight = dst.width, dst.height
	}
	for level := passes - 1; level >= 0; level-- {
		src := r.postBloom[level+1]
		dst := r.postBloom[level]
		uniformIndex := index + passes + 1 + (passes - 1 - level)
		postUniformBuffer(r, uniformIndex, func(data []byte) {
			putPostF32(data, 0, float64(src.width))
			putPostF32(data, 4, float64(src.height))
			putPostF32(data, 8, float64(dst.width))
			putPostF32(data, 12, float64(dst.height))
		})
		r.encodePostPass(encoder, postPipelineBlurUp, postSourceBloom+level+1, src.view, dst.view, uniformIndex)
	}
	combineIndex := index + 2*passes + 1
	postUniformBuffer(r, combineIndex, func(data []byte) { putPostF32(data, 0, strength) })
	r.encodePostCombine2(encoder, postPipelineBlurCombine, sourceID, source.view, r.postBloom[0].view, destination, combineIndex)
	return nil
}

// encodePostCombine2 is encodePostCombine generalized over the pipeline name,
// since Blur's combine shares the two-texture bind layout but a different
// pipeline than bloom's combine.
func (r *webgpuRenderer) encodePostCombine2(encoder js.Value, pipelineName string, sourceID int, source, second, destination js.Value, uniformIndex int) {
	pass := encoder.Call("beginRenderPass", map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":       destination,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
		}},
	})
	pipeline := r.postPipelines[pipelineName]
	key := postBindKey{pipeline: pipelineName, source: sourceID, bloom: postSourceBloom, uniform: uniformIndex}
	bindGroup := r.cachedBindGroup(key, func() js.Value {
		return r.device.Call("createBindGroup", map[string]any{
			"layout": pipeline.Call("getBindGroupLayout", 0),
			"entries": []any{
				map[string]any{"binding": 0, "resource": source},
				map[string]any{"binding": 1, "resource": second},
				map[string]any{"binding": 2, "resource": r.postSampler},
				map[string]any{"binding": 3, "resource": map[string]any{"buffer": r.postUniforms[uniformIndex]}},
			},
		})
	})
	pass.Call("setPipeline", pipeline)
	pass.Call("setBindGroup", 0, bindGroup)
	pass.Call("draw", 3, 1, 0, 0)
	pass.Call("end")
}

// cachedBindGroup returns the bind group for key, building it on the first
// use of that combination. Every referenced object is stable between frames.
func (r *webgpuRenderer) cachedBindGroup(key postBindKey, build func() js.Value) js.Value {
	if group, ok := r.postBindGroups[key]; ok {
		return group
	}
	if r.postBindGroups == nil {
		r.postBindGroups = map[postBindKey]js.Value{}
	}
	group := build()
	r.postBindGroups[key] = group
	return group
}

func (r *webgpuRenderer) postBindGroup(key postBindKey, source js.Value) js.Value {
	return r.cachedBindGroup(key, func() js.Value {
		pipeline := r.postPipelines[key.pipeline]
		entries := []any{
			map[string]any{"binding": 0, "resource": source},
			map[string]any{"binding": 1, "resource": r.postSampler},
		}
		if key.uniform >= 0 {
			entries = append(entries, map[string]any{"binding": 2, "resource": map[string]any{"buffer": r.postUniforms[key.uniform]}})
		}
		return r.device.Call("createBindGroup", map[string]any{
			"layout":  pipeline.Call("getBindGroupLayout", 0),
			"entries": entries,
		})
	})
}

func (r *webgpuRenderer) encodePostPass(encoder js.Value, pipelineName string, sourceID int, source, destination js.Value, uniformIndex int) {
	pass := encoder.Call("beginRenderPass", map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":       destination,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
		}},
	})
	r.drawPostFullscreen(pass, pipelineName, sourceID, source, uniformIndex)
	pass.Call("end")
}

func (r *webgpuRenderer) encodePostCombine(encoder js.Value, sourceID int, source, bloom, destination js.Value, uniformIndex int) {
	pass := encoder.Call("beginRenderPass", map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":       destination,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
		}},
	})
	pipeline := r.postPipelines[postPipelineBloomCombine]
	key := postBindKey{pipeline: postPipelineBloomCombine, source: sourceID, bloom: postSourceBloom, uniform: uniformIndex}
	bindGroup := r.cachedBindGroup(key, func() js.Value {
		return r.device.Call("createBindGroup", map[string]any{
			"layout": pipeline.Call("getBindGroupLayout", 0),
			"entries": []any{
				map[string]any{"binding": 0, "resource": source},
				map[string]any{"binding": 1, "resource": bloom},
				map[string]any{"binding": 2, "resource": r.postSampler},
				map[string]any{"binding": 3, "resource": map[string]any{"buffer": r.postUniforms[uniformIndex]}},
			},
		})
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

func (r *webgpuRenderer) encodeBloom(encoder js.Value, source webgpuPostTarget, sourceID int, destination js.Value, effect render.Bloom, index int) error {
	if effect.Intensity == 0 {
		return nil
	}
	passes := postBloomPasses(effect)
	if err := r.ensureBloomTargets(passes); err != nil {
		return err
	}
	postUniformBuffer(r, index, func(dst []byte) {
		putPostF32(dst, 0, effect.Threshold)
	})
	r.encodePostPass(encoder, postPipelineBloomBright, sourceID, source.view, r.postBloom[0].view, index)
	for level := 1; level <= passes; level++ {
		src := r.postBloom[level-1]
		dst := r.postBloom[level]
		uniformIndex := index + level
		postUniformBuffer(r, uniformIndex, func(data []byte) {
			putPostF32(data, 0, float64(src.width))
			putPostF32(data, 4, float64(src.height))
			putPostF32(data, 8, float64(dst.width))
			putPostF32(data, 12, float64(dst.height))
		})
		r.encodePostPass(encoder, postPipelineBloomDown, postSourceBloom+level-1, src.view, dst.view, uniformIndex)
	}
	for level := passes - 1; level >= 0; level-- {
		src := r.postBloom[level+1]
		dst := r.postBloom[level]
		uniformIndex := index + passes + 1 + (passes - 1 - level)
		postUniformBuffer(r, uniformIndex, func(data []byte) {
			putPostF32(data, 0, float64(src.width))
			putPostF32(data, 4, float64(src.height))
			putPostF32(data, 8, float64(dst.width))
			putPostF32(data, 12, float64(dst.height))
		})
		r.encodePostPass(encoder, postPipelineBloomUp, postSourceBloom+level+1, src.view, dst.view, uniformIndex)
	}
	combineIndex := index + 2*passes + 1
	postUniformBuffer(r, combineIndex, func(data []byte) {
		putPostF32(data, 0, effect.Intensity)
	})
	r.encodePostCombine(encoder, sourceID, source.view, r.postBloom[0].view, destination, combineIndex)
	return nil
}

func (r *webgpuRenderer) drawPostFullscreen(pass js.Value, pipelineName string, sourceID int, source js.Value, uniformIndex int) {
	pipeline := r.postPipelines[pipelineName]
	key := postBindKey{pipeline: pipelineName, source: sourceID, bloom: -1, uniform: uniformIndex}
	pass.Call("setPipeline", pipeline)
	pass.Call("setBindGroup", 0, r.postBindGroup(key, source))
	pass.Call("draw", 3, 1, 0, 0)
}

func (r *webgpuRenderer) renderPostFramePasses(
	camera render.Camera,
	runs []gpuDrawRun,
	plans []gpuPassPlan,
	chain render.PostChain,
	w, h int,
) error {
	if err := r.ensurePostTargets(w, h); err != nil {
		return err
	}
	encoder := r.device.Call("createCommandEncoder")
	textureView := r.context.Call("getCurrentTexture").Call("createView")
	sampleTiming := r.timing != nil
	if r.particles != nil {
		if sampleTiming {
			r.particles.armTiming(r.timing)
		}
		particleRuns := make([]gpuParticleRun, 0)
		for _, run := range runs {
			if run.kind == tess.DrawRunParticles {
				plan := plans[run.pass]
				particleRuns = append(particleRuns, gpuParticleRun{
					command: run.particles,
					camera:  tess.CameraForLayer(plan.camera, plan.layerCameras, run.layer),
					width:   plan.width,
					height:  plan.height,
				})
			}
		}
		r.particles.encodeSims(encoder, particleRuns, w, h)
	}

	firstScreen, lastScreen := firstAndLastScreenPass(plans)
	for passIndex, plan := range plans {
		if plan.target != 0 {
			r.encodeTargetPass(encoder, passIndex, plan, runs)
			continue
		}
		loadOp := "load"
		if passIndex == firstScreen {
			loadOp = "clear"
		}
		attachment := map[string]any{
			"view": r.msaaView, "resolveTarget": r.postSceneA.view,
			"loadOp": loadOp, "storeOp": "store", "clearValue": clearValue(r.clear),
		}
		if passIndex == lastScreen {
			attachment["storeOp"] = "discard"
		}
		if r.msaaSamples <= 1 {
			attachment = map[string]any{
				"view": r.postSceneA.view, "loadOp": loadOp,
				"storeOp": "store", "clearValue": clearValue(r.clear),
			}
		}
		desc := map[string]any{"colorAttachments": []any{attachment}}
		// The GPU render window ends at the last screen pass; the post
		// effect chain stays outside the measurement.
		if sampleTiming {
			switch {
			case passIndex == firstScreen && passIndex == lastScreen:
				desc["timestampWrites"] = r.timing.renderTimestampWrites()
			case passIndex == firstScreen:
				desc["timestampWrites"] = r.timing.renderBeginTimestampWrites()
			case passIndex == lastScreen:
				desc["timestampWrites"] = r.timing.renderEndTimestampWrites()
			}
		}
		pass := encoder.Call("beginRenderPass", desc)
		r.drawPostPlanRuns(pass, runs, plans, passIndex, chain.BelowLayer, true, w, h)
		pass.Call("end")
	}

	// Without MSAA and without runs above the chain, the last active effect
	// renders directly to the swapchain and the final blit pass disappears.
	lastActive := -1
	for i, effect := range chain.Effects {
		if postEffectEncodes(effect) {
			lastActive = i
		}
	}
	directFinal := r.msaaSamples <= 1 && lastActive >= 0 &&
		!hasPostRuns(runs, plans, false, chain.BelowLayer)

	source, destination := r.postSceneA, r.postSceneB
	sourceID, destID := postSourceSceneA, postSourceSceneB
	uniformIndex := 0
	for i, effect := range chain.Effects {
		destView := destination.view
		if directFinal && i == lastActive {
			destView = textureView
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
		pipelineName := r.postParams(uniformIndex, effect, camera, chain.Time, w, h)
		uniformIndex++
		if pipelineName == "" {
			continue
		}
		r.encodePostPass(encoder, pipelineName, sourceID, source.view, destView, uniformIndex-1)
		source, destination = destination, source
		sourceID, destID = destID, sourceID
	}

	if !directFinal {
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
		r.drawPostFullscreen(finalPass, postPipelineFinalBlit, sourceID, source.view, -1)
		r.drawPostRuns(finalPass, runs, plans, chain.BelowLayer, false, w, h)
		finalPass.Call("end")
	}
	timingSlot := -1
	if sampleTiming {
		timedCompute := r.particles != nil && r.particles.timingWrote
		timingSlot = r.timing.encodeResolve(encoder, timedCompute)
	}
	commandBuffer := encoder.Call("finish")
	r.queue.Call("submit", []any{commandBuffer})
	if timingSlot >= 0 {
		r.timing.readback(timingSlot)
	}
	if r.particles != nil {
		r.particles.endFrame()
	}
	r.signalReadyAfterPresent()
	return nil
}

// postEffectEncodes mirrors the skip rules of the encode loop: a zero-intensity
// bloom and unknown effect types produce no pass.
func postEffectEncodes(effect render.PostEffect) bool {
	switch effect := effect.(type) {
	case render.Bloom:
		return effect.Intensity != 0
	case render.Blur:
		return clampPostStrength(effect.Strength) != 0
	case render.Vignette, render.ColorGrade, render.Pixelate,
		render.ChromaticAberration, render.Shockwave, render.LensRain,
		render.Fade, render.CircleWipe, render.Dissolve, render.FilmGrain,
		render.Scanlines, render.RadialBlur:
		return true
	default:
		return false
	}
}

// clampPostStrength mirrors platform/raster/postfx.go clamp01 for the one
// Blur field the encode loop must gate on before spending a pyramid pass.
func clampPostStrength(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 1
	}
	return value
}

func hasPostRuns(runs []gpuDrawRun, plans []gpuPassPlan, below bool, belowLayer int) bool {
	for _, run := range runs {
		if postRunVisible(run, len(plans)-1, below, belowLayer) {
			return true
		}
	}
	return false
}

func postRunVisible(run gpuDrawRun, basePass int, below bool, belowLayer int) bool {
	if run.pass < basePass {
		return below
	}
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
	plans []gpuPassPlan,
	belowLayer int,
	below bool,
	w, h int,
) {
	scissor := webGPUScissorState{}
	for _, run := range runs {
		if !postRunVisible(run, len(plans)-1, below, belowLayer) {
			continue
		}
		plan := plans[run.pass]
		effective := tess.CameraForLayer(plan.camera, plan.layerCameras, run.layer)
		r.drawRun(pass, run, effective, passViewport(plan.camera, plan.extra, w, h), &scissor, w, h, false, false, nil)
	}
}

func (r *webgpuRenderer) drawPostPlanRuns(
	pass js.Value,
	runs []gpuDrawRun,
	plans []gpuPassPlan,
	passIndex, belowLayer int,
	below bool,
	w, h int,
) {
	plan := plans[passIndex]
	scissor := webGPUScissorState{}
	for _, run := range runs {
		if run.pass != passIndex || !postRunVisible(run, len(plans)-1, below, belowLayer) {
			continue
		}
		effective := tess.CameraForLayer(plan.camera, plan.layerCameras, run.layer)
		r.drawRun(pass, run, effective, passViewport(plan.camera, plan.extra, plan.width, plan.height), &scissor, plan.width, plan.height, false, false, nil)
	}
}

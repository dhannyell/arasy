//go:build js && wasm

package wasm

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"syscall/js"
	"time"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/platform/wasm/gpu"
	"github.com/dhannyell/frameforge/platform/wasm/tess"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

var errWebGPUUnavailable = errors.New("webgpu unavailable")

// msaaSampleCount is the multisample count for the main render pass. 4 is the
// value WebGPU guarantees is supported on every adapter; it smooths geometry
// edges (circles, rounded corners, polygon silhouettes) that would otherwise
// rasterize with hard, aliased edges at one sample per pixel.
const msaaSampleCount = 4

type webgpuRenderer struct {
	canvas   js.Value
	clear    render.Color
	manifest asset.Manifest

	gpu     js.Value
	device  js.Value
	queue   js.Value
	dev     gpu.Device
	context js.Value
	format  js.Value

	msaaTexture js.Value
	msaaView    js.Value
	msaaWidth   int
	msaaHeight  int

	solidPipeline    js.Value
	textPipeline     js.Value
	texturedPipeline js.Value
	// mipPipeline renders a fullscreen triangle sampling one mip level into
	// the next. Boot-time only, used by generateMipmaps.
	mipPipeline js.Value

	// texSampler is shared by every declared texture: linear filtering in
	// every dimension plus anisotropy, unlike the font atlas's nearest
	// sampler which wants pixel-sharp text.
	texSampler js.Value
	// mipSampler is used only while generating mip chains, sampling a
	// single source level at a time.
	mipSampler js.Value

	solidBuffer js.Value
	textBuffer  js.Value
	texBuffer   js.Value
	solidCap    int
	textCap     int
	texCap      int

	fontProvider *fontAtlasProvider
	textAtlases  map[tess.AtlasKey]gpuTextAtlas

	// texSizes maps TextureID to pixel dimensions for the textured
	// pipeline, populated from the asset manifest in newWebGPU.
	texSizes tess.TexSizes

	// textures maps TextureID to its GPU texture/view/bind group, indexed
	// like texSizes (index 0 unused, TextureID 0 means "missing texture").
	textures  []gpuTexture
	particles *gpuParticles
	timing    *gpuTiming // nil when "timestamp-query" was not granted

	// Offscreen particle compositing, active when particleScale > 0: each
	// contiguous run of particle commands renders into one pooled
	// single-sample target and is composited back at its painter-order
	// position by a fullscreen triangle.
	particleScale     float64
	compositePipeline js.Value
	compositeSampler  js.Value
	particleTargets   []particleTarget
	particleTargetW   int
	particleTargetH   int

	ready        bool
	readyPending bool
	errorCb      js.Func
	lostCb       js.Func
}

func newWebGPU(canvas js.Value, clear render.Color, manifest asset.Manifest, wantGPUTiming bool, particleScale float64, log *slog.Logger) (*webgpuRenderer, error) {
	fontProvider, err := newFontAtlasProvider(manifest)
	if err != nil {
		return nil, err
	}
	gpu := js.Global().Get("navigator").Get("gpu")
	if gpu.IsUndefined() || gpu.IsNull() {
		return nil, errWebGPUUnavailable
	}

	adapter, err := awaitPromise(gpu.Call("requestAdapter"))
	if err != nil {
		return nil, fmt.Errorf("request webgpu adapter: %w", err)
	}
	if adapter.IsUndefined() || adapter.IsNull() {
		return nil, errWebGPUUnavailable
	}

	// Timestamp queries power the particle GPU-time telemetry, so the feature
	// is only requested when the host has a FrameMetrics consumer; without
	// one, per-frame sampling would be pure overhead for every game.
	deviceOptions := map[string]any{}
	if wantGPUTiming && adapter.Get("features").Call("has", "timestamp-query").Bool() {
		deviceOptions["requiredFeatures"] = []any{"timestamp-query"}
	}
	device, err := awaitPromise(adapter.Call("requestDevice", deviceOptions))
	if err != nil {
		return nil, fmt.Errorf("request webgpu device: %w", err)
	}

	context := canvas.Call("getContext", "webgpu")
	if context.IsUndefined() || context.IsNull() {
		return nil, errWebGPUUnavailable
	}

	format := gpu.Call("getPreferredCanvasFormat")
	context.Call("configure", map[string]any{
		"device":    device,
		"format":    format,
		"alphaMode": "opaque",
	})

	r := &webgpuRenderer{
		canvas:        canvas,
		clear:         clear,
		manifest:      manifest,
		gpu:           gpu,
		device:        device,
		queue:         device.Get("queue"),
		context:       context,
		format:        format,
		fontProvider:  fontProvider,
		textAtlases:   make(map[tess.AtlasKey]gpuTextAtlas),
		particleScale: particleScale,
	}
	r.dev = newJSDevice(r.device, r.queue)
	r.observeDeviceErrors(log)

	r.solidPipeline = r.createSolidPipeline()
	r.textPipeline = r.createTextPipeline()
	r.texturedPipeline = r.createTexturedPipeline()
	r.mipPipeline = r.createMipPipeline()
	r.texSampler = r.createTextureSampler()
	r.mipSampler = r.createMipSampler()
	r.solidBuffer, r.solidCap = r.createVertexBuffer(64 * 1024)
	r.textBuffer, r.textCap = r.createVertexBuffer(64 * 1024)
	r.texBuffer, r.texCap = r.createVertexBuffer(64 * 1024)

	r.textures, r.texSizes = r.buildTextures(manifest)
	r.particles = newGpuParticles(r, particleScale > 0)
	if particleScale > 0 {
		r.compositePipeline = r.createCompositePipeline()
		r.compositeSampler = r.device.Call("createSampler", map[string]any{
			"magFilter": "linear",
			"minFilter": "linear",
		})
	}
	r.timing = newGpuTiming(device)

	return r, nil
}

func (r *webgpuRenderer) particleGPUBufferBytes() uint64 {
	if r.particles == nil {
		return 0
	}
	return r.particles.bufferBytes()
}

func (r *webgpuRenderer) observeDeviceErrors(log *slog.Logger) {
	r.errorCb = js.FuncOf(func(this js.Value, args []js.Value) any {
		message := "unknown WebGPU error"
		if len(args) > 0 {
			err := args[0].Get("error")
			if !err.IsUndefined() && !err.IsNull() {
				message = err.Get("message").String()
			}
		}
		log.Error("webgpu uncaptured error", "error", message)
		return nil
	})
	r.device.Call("addEventListener", "uncapturederror", r.errorCb)

	r.lostCb = js.FuncOf(func(this js.Value, args []js.Value) any {
		reason := "unknown"
		message := ""
		if len(args) > 0 {
			info := args[0]
			reason = info.Get("reason").String()
			message = info.Get("message").String()
		}
		log.Error("webgpu device lost", "reason", reason, "message", message)
		return nil
	})
	r.device.Get("lost").Call("then", r.lostCb)
}

func (r *webgpuRenderer) Render(frame runtime.Frame) {
	w := r.canvas.Get("width").Int()
	h := r.canvas.Get("height").Int()
	if w <= 0 || h <= 0 {
		return
	}

	runs := tess.Build(frame, w, h, r.fontProvider, r.texSizes)

	var solidBytes []byte
	var texBytes []byte
	var textBytes []byte
	drawRuns := make([]gpuDrawRun, 0, len(runs))
	solidOffset := 0
	texOffset := 0
	textOffset := 0
	for _, run := range runs {
		switch run.Kind {
		case tess.DrawRunSolid:
			if len(run.Solid.Vertices) == 0 {
				continue
			}
			solidBytes = append(solidBytes, tess.Bytes(run.Solid)...)
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:   tess.DrawRunSolid,
				offset: solidOffset,
				count:  len(run.Solid.Vertices),
			})
			solidOffset += len(run.Solid.Vertices)
		case tess.DrawRunTextured:
			if len(run.Textured.Vertices) == 0 {
				continue
			}
			texBytes = append(texBytes, tess.TexBytes(run.Textured)...)
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:    tess.DrawRunTextured,
				texture: run.Textured.Texture,
				offset:  texOffset,
				count:   len(run.Textured.Vertices),
			})
			texOffset += len(run.Textured.Vertices)
		case tess.DrawRunText:
			if len(run.Text.Vertices) == 0 {
				continue
			}
			textBytes = append(textBytes, tess.TextBytes(run.Text)...)
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:     tess.DrawRunText,
				atlasKey: run.AtlasKey,
				offset:   textOffset,
				count:    len(run.Text.Vertices),
			})
			textOffset += len(run.Text.Vertices)
		case tess.DrawRunParticles:
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:      tess.DrawRunParticles,
				particles: run.Particles,
			})
		}
	}

	r.ensureSolidBuffer(len(solidBytes))
	r.ensureTextBuffer(len(textBytes))

	if len(solidBytes) > 0 {
		r.queue.Call("writeBuffer", r.solidBuffer, 0, bytesToJS(solidBytes))
	}
	if len(textBytes) > 0 {
		r.queue.Call("writeBuffer", r.textBuffer, 0, bytesToJS(textBytes))
	}

	r.ensureTexBuffer(len(texBytes))
	if len(texBytes) > 0 {
		r.queue.Call("writeBuffer", r.texBuffer, 0, bytesToJS(texBytes))
	}

	r.ensureMSAATarget(w, h)

	encoder := r.device.Call("createCommandEncoder")
	textureView := r.context.Call("getCurrentTexture").Call("createView")

	// Particle simulation writes indirect draw arguments and must complete
	// before the render pass reads them. All passes share this encoder, so GPU
	// command order provides the dependency without a readback or fence.
	if r.timing != nil {
		r.particles.armTiming(r.timing)
	}
	// Contiguous particle runs form composite groups: with offscreen
	// compositing enabled, each group renders into one pooled target and is
	// composited once, at the painter-order position of its first run.
	var particleRuns []render.DrawParticles
	var groupCmds [][]render.DrawParticles
	inGroup := false
	for i := range drawRuns {
		if drawRuns[i].kind != tess.DrawRunParticles {
			inGroup = false
			continue
		}
		particleRuns = append(particleRuns, drawRuns[i].particles)
		if !inGroup {
			groupCmds = append(groupCmds, nil)
			inGroup = true
		}
		drawRuns[i].group = len(groupCmds) - 1
		groupCmds[len(groupCmds)-1] = append(groupCmds[len(groupCmds)-1], drawRuns[i].particles)
	}

	r.particles.encodeSims(encoder, particleRuns, frame.Camera, w, h)
	// Sample GPU time only on frames whose first particle compute pass wrote
	// the begin timestamp; otherwise query 0 would hold a stale value.
	sampleTiming := r.timing != nil && r.particles.timingWrote

	offscreenParticles := r.particleScale > 0 && len(groupCmds) > 0
	if offscreenParticles {
		r.encodeParticleOffscreen(encoder, groupCmds, frame.Camera, w, h, sampleTiming)
	}

	renderPassDesc := map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":          r.msaaView,
			"resolveTarget": textureView,
			"loadOp":        "clear",
			"storeOp":       "discard",
			"clearValue":    clearValue(r.clear),
		}},
	}
	if sampleTiming {
		// With offscreen compositing the draw interval opens at the first
		// offscreen particle pass, so the main pass only closes it.
		if offscreenParticles {
			renderPassDesc["timestampWrites"] = r.timing.renderEndTimestampWrites()
		} else {
			renderPassDesc["timestampWrites"] = r.timing.renderTimestampWrites()
		}
	}
	pass := encoder.Call("beginRenderPass", renderPassDesc)

	// Runs are submitted in painter order. Each pipeline has its own packed
	// vertex buffer, so the run stores a firstVertex offset into that buffer.
	lastComposited := -1
	for _, run := range drawRuns {
		switch run.kind {
		case tess.DrawRunSolid:
			pass.Call("setPipeline", r.solidPipeline)
			pass.Call("setVertexBuffer", 0, r.solidBuffer)
			pass.Call("draw", run.count, 1, run.offset, 0)
		case tess.DrawRunTextured:
			gt, ok := r.textureFor(run.texture)
			if !ok {
				continue
			}
			pass.Call("setPipeline", r.texturedPipeline)
			pass.Call("setVertexBuffer", 0, r.texBuffer)
			pass.Call("setBindGroup", 0, gt.bindGroup)
			pass.Call("draw", run.count, 1, run.offset, 0)
		case tess.DrawRunText:
			atlas, ok := r.textAtlasFor(run.atlasKey)
			if !ok {
				continue
			}
			pass.Call("setPipeline", r.textPipeline)
			pass.Call("setBindGroup", 0, atlas.bindGroup)
			pass.Call("setVertexBuffer", 0, r.textBuffer)
			pass.Call("draw", run.count, 1, run.offset, 0)
		case tess.DrawRunParticles:
			if !offscreenParticles {
				r.particles.draw(pass, run.particles, frame.Camera, w, h)
				continue
			}
			if run.group == lastComposited {
				continue
			}
			lastComposited = run.group
			pass.Call("setPipeline", r.compositePipeline)
			pass.Call("setBindGroup", 0, r.particleTargets[run.group].bindGroup)
			pass.Call("draw", 3, 1, 0, 0)
		}
	}

	pass.Call("end")
	timingSlot := -1
	if sampleTiming {
		timingSlot = r.timing.encodeResolve(encoder)
	}
	commandBuffer := encoder.Call("finish")
	r.queue.Call("submit", []any{commandBuffer})
	if timingSlot >= 0 {
		r.timing.readback(timingSlot)
	}
	r.particles.endFrame()
	r.signalReadyAfterPresent()
}

// particleGPUTimings reports the most recent timestamp-query sample: the
// particle prepare pass (kick + emit), the simulate pass (including the pass
// boundary before it) and the main render pass. ok is false until the first
// sample resolves, or always on adapters without "timestamp-query".
func (r *webgpuRenderer) particleGPUTimings() (prepare, simulate, render time.Duration, ok bool) {
	if r.timing == nil || !r.timing.valid {
		return 0, 0, 0, false
	}
	return r.timing.prepare, r.timing.simulate, r.timing.render, true
}

// encodeParticleOffscreen records one single-sample render pass per composite
// group, drawing its particle runs into a pooled target cleared to
// transparent black. Painter order inside the group is preserved; ordering
// against the rest of the scene is restored by compositing at the group's
// position in the main pass.
func (r *webgpuRenderer) encodeParticleOffscreen(
	encoder js.Value,
	groups [][]render.DrawParticles,
	camera render.Camera,
	w, h int,
	sampleTiming bool,
) {
	ow, oh := particleTargetDims(w, h, r.particleScale)
	r.ensureParticleTargets(len(groups), ow, oh)
	for gi, cmds := range groups {
		desc := map[string]any{
			"colorAttachments": []any{map[string]any{
				"view":       r.particleTargets[gi].view,
				"loadOp":     "clear",
				"storeOp":    "store",
				"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 0},
			}},
		}
		if sampleTiming && gi == 0 {
			desc["timestampWrites"] = r.timing.renderBeginTimestampWrites()
		}
		pass := encoder.Call("beginRenderPass", desc)
		for _, cmd := range cmds {
			r.particles.drawTo(pass, cmd, camera, r.particleScale, ow, oh, true)
		}
		pass.Call("end")
	}
}

func particleTargetDims(w, h int, scale float64) (int, int) {
	return max(1, int(math.Ceil(float64(w)*scale))), max(1, int(math.Ceil(float64(h)*scale)))
}

// ensureParticleTargets grows the pooled offscreen targets to n and rebuilds
// the pool when the composite dimensions change. The pool never shrinks: its
// high-water mark is the largest number of interleaved particle groups a
// frame has produced.
func (r *webgpuRenderer) ensureParticleTargets(n, ow, oh int) {
	if ow != r.particleTargetW || oh != r.particleTargetH {
		for _, target := range r.particleTargets {
			target.texture.Call("destroy")
		}
		r.particleTargets = nil
		r.particleTargetW, r.particleTargetH = ow, oh
	}
	layout := r.compositePipeline.Call("getBindGroupLayout", 0)
	for len(r.particleTargets) < n {
		texture := r.device.Call("createTexture", map[string]any{
			"size":   []any{ow, oh},
			"format": r.format,
			"usage":  gpu.TextureUsageRenderAttachment | gpu.TextureUsageTextureBinding,
		})
		view := texture.Call("createView")
		bindGroup := r.device.Call("createBindGroup", map[string]any{
			"layout": layout,
			"entries": []any{
				map[string]any{"binding": 0, "resource": view},
				map[string]any{"binding": 1, "resource": r.compositeSampler},
			},
		})
		r.particleTargets = append(r.particleTargets, particleTarget{texture: texture, view: view, bindGroup: bindGroup})
	}
}

// createCompositePipeline builds the fullscreen-triangle pipeline that blends
// one particle offscreen target into the MSAA main pass with premultiplied
// source-over.
func (r *webgpuRenderer) createCompositePipeline() js.Value {
	module := r.device.Call("createShaderModule", map[string]any{"code": particlesCompositeWGSL})
	return r.device.Call("createRenderPipeline", map[string]any{
		"layout": "auto",
		"vertex": map[string]any{"module": module, "entryPoint": "vs_main"},
		"fragment": map[string]any{
			"module":     module,
			"entryPoint": "fs_main",
			"targets":    []any{map[string]any{"format": r.format, "blend": gpu.PremultipliedBlend()}},
		},
		"primitive":   map[string]any{"topology": "triangle-list"},
		"multisample": map[string]any{"count": msaaSampleCount},
	})
}

// signalReadyAfterPresent marks the canvas ready only after the browser has had
// two animation frames in which to present the first submission. queue.submit
// only enqueues work, while onSubmittedWorkDone may remain pending indefinitely
// on continuously-rendering software GPU backends.
func (r *webgpuRenderer) signalReadyAfterPresent() {
	if r.ready || r.readyPending {
		return
	}
	r.readyPending = true

	var firstFrame js.Func
	firstFrame = js.FuncOf(func(this js.Value, args []js.Value) any {
		firstFrame.Release()

		var secondFrame js.Func
		secondFrame = js.FuncOf(func(this js.Value, args []js.Value) any {
			secondFrame.Release()
			r.canvas.Call("setAttribute", "data-frameforge-ready", "webgpu")
			r.ready = true
			return nil
		})
		js.Global().Call("requestAnimationFrame", secondFrame)
		return nil
	})
	js.Global().Call("requestAnimationFrame", firstFrame)
}

type gpuDrawRun struct {
	kind      tess.DrawRunKind
	texture   render.TextureID
	atlasKey  tess.AtlasKey
	particles render.DrawParticles
	offset    int
	count     int
	// group indexes the composite group this particle run belongs to when
	// offscreen compositing is active; contiguous particle runs share one.
	group int
}

// particleTarget is one pooled offscreen target plus its composite bind group.
type particleTarget struct {
	texture   js.Value
	view      js.Value
	bindGroup js.Value
}

func (r *webgpuRenderer) createSolidPipeline() js.Value {
	module := r.dev.CreateShaderModule(solidWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.SolidPipelineDesc(module, r.format.String(), msaaSampleCount)))
}

func (r *webgpuRenderer) createTextPipeline() js.Value {
	module := r.dev.CreateShaderModule(textWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.TextPipelineDesc(module, r.format.String(), msaaSampleCount)))
}

func (r *webgpuRenderer) createTexturedPipeline() js.Value {
	module := r.dev.CreateShaderModule(texturedWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.TexturedPipelineDesc(module, r.format.String(), msaaSampleCount)))
}

// createMipPipeline builds the internal pipeline used by generateMipmaps: a
// fullscreen triangle (no vertex buffer) sampling one mip level into the
// next, one render pass per level.
func (r *webgpuRenderer) createMipPipeline() js.Value {
	module := r.dev.CreateShaderModule(mipmapWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.MipPipelineDesc(module)))
}

// createTextureSampler builds the sampler shared by every declared texture.
// The WebGPU spec requires magFilter, minFilter and mipmapFilter to all be
// "linear" whenever maxAnisotropy > 1.
func (r *webgpuRenderer) createTextureSampler() js.Value {
	return jsHandle(r.dev.CreateSampler(gpu.TextureSamplerDesc()))
}

// createMipSampler builds the sampler used only while generating mip
// chains: each sample reads a single source level, so no anisotropy or
// mipmap filtering is needed.
func (r *webgpuRenderer) createMipSampler() js.Value {
	return jsHandle(r.dev.CreateSampler(gpu.MipSamplerDesc()))
}

type gpuTextAtlas struct {
	atlas     *tess.Atlas
	texture   js.Value
	bindGroup js.Value
}

func (r *webgpuRenderer) textAtlasFor(key tess.AtlasKey) (gpuTextAtlas, bool) {
	atlas, err := r.fontProvider.AtlasFor(key.Font, key.PX, "")
	if err != nil || atlas == nil {
		return gpuTextAtlas{}, false
	}
	if cached, ok := r.textAtlases[key]; ok && cached.atlas == atlas {
		return cached, true
	} else if ok && !cached.texture.IsUndefined() && !cached.texture.IsNull() {
		cached.texture.Call("destroy")
	}
	gpuAtlas := r.createTextAtlas(atlas)
	r.textAtlases[key] = gpuAtlas
	return gpuAtlas, true
}

func (r *webgpuRenderer) createTextAtlas(atlas *tess.Atlas) gpuTextAtlas {
	bounds := atlas.Image.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	texture := r.dev.CreateTexture(gpu.R8TextureDesc(width, height))
	r.dev.WriteTexture(map[string]any{"texture": texture}, atlas.Image.Pix, gpu.TextureWriteLayout(width, height), gpu.TextureWriteSize(width, height))
	view := r.dev.CreateView(texture, nil)
	sampler := r.dev.CreateSampler(gpu.NearestSamplerDesc())
	layout := r.dev.BindGroupLayout(r.textPipeline, 0)
	bindGroup := r.dev.CreateBindGroup(gpu.TextureBindGroupDesc(layout, view, sampler))
	return gpuTextAtlas{atlas: atlas, texture: jsHandle(texture), bindGroup: jsHandle(bindGroup)}
}

func (r *webgpuRenderer) createVertexBuffer(size int) (js.Value, int) {
	return jsHandle(r.dev.CreateBuffer(gpu.VertexBufferDesc(size))), size
}

// ensureMSAATarget (re)creates the multisampled color target when the canvas
// dimensions change.
func (r *webgpuRenderer) ensureMSAATarget(w, h int) {
	if r.msaaWidth == w && r.msaaHeight == h &&
		!r.msaaTexture.IsUndefined() && !r.msaaTexture.IsNull() {
		return
	}
	if !r.msaaTexture.IsUndefined() && !r.msaaTexture.IsNull() {
		r.msaaTexture.Call("destroy")
	}
	r.msaaTexture = jsHandle(r.dev.CreateTexture(gpu.MSAATextureDesc(w, h, r.format.String(), msaaSampleCount)))
	r.msaaView = r.msaaTexture.Call("createView")
	r.msaaWidth, r.msaaHeight = w, h
}

func (r *webgpuRenderer) ensureSolidBuffer(size int) {
	if size <= r.solidCap {
		return
	}

	newCap := growBufferCap(r.solidCap, size)
	r.solidBuffer, r.solidCap = r.createVertexBuffer(newCap)
}

func (r *webgpuRenderer) ensureTextBuffer(size int) {
	if size <= r.textCap {
		return
	}

	newCap := growBufferCap(r.textCap, size)
	r.textBuffer, r.textCap = r.createVertexBuffer(newCap)
}

func (r *webgpuRenderer) ensureTexBuffer(size int) {
	if size <= r.texCap {
		return
	}

	newCap := growBufferCap(r.texCap, size)
	r.texBuffer, r.texCap = r.createVertexBuffer(newCap)
}

func awaitPromise(promise js.Value) (js.Value, error) {
	resolveChan := make(chan js.Value, 1)
	rejectChan := make(chan js.Value, 1)

	onResolve := js.FuncOf(func(this js.Value, args []js.Value) any {
		var val js.Value
		if len(args) > 0 {
			val = args[0]
		}
		resolveChan <- val
		return nil
	})
	defer onResolve.Release()

	onReject := js.FuncOf(func(this js.Value, args []js.Value) any {
		var val js.Value
		if len(args) > 0 {
			val = args[0]
		}
		rejectChan <- val
		return nil
	})
	defer onReject.Release()

	promise.Call("then", onResolve).Call("catch", onReject)

	select {
	case res := <-resolveChan:
		return res, nil
	case err := <-rejectChan:
		return js.Undefined(), fmt.Errorf("JS Error: %s", err.Call("toString").String())
	}
}

func growBufferCap(current, required int) int {
	if current <= 0 {
		current = 64 * 1024
	}

	for current < required {
		current *= 2
	}

	return current
}

func clearValue(clear render.Color) map[string]any {
	return map[string]any{
		"r": clear.R * clear.A,
		"g": clear.G * clear.A,
		"b": clear.B * clear.A,
		"a": 1,
	}
}

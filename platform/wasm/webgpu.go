//go:build js && wasm

package wasm

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"syscall/js"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/render/gpu/fontatlas"
	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/wasm/gpu"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
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

	solidPipeline       js.Value
	textGlyphPipeline   js.Value
	texturedPipeline    js.Value
	spritePipeline      js.Value
	spriteArrayPipeline js.Value
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

	solidBuffer          js.Value
	textGlyphBuffer      js.Value
	textLabelBuffer      js.Value
	texBuffer            js.Value
	spriteBuffer         js.Value
	spriteUniform        js.Value
	solidCap             int
	textGlyphCap         int
	textLabelCap         int
	texCap               int
	spriteCap            int
	spriteUniformScratch [tess.SpriteUniformSize]byte
	spriteArrayTexture   js.Value
	spriteArrayBindGroup js.Value
	spriteArrayEnabled   bool

	// Retained sprite batches: registry keyed by game-chosen ID, the shared
	// animated-uniform arena, and the lazily created 1x1 magenta placeholder
	// view. Content is render-only.
	spriteBatches                map[render.SpriteBatchID]*gpuSpriteBatch
	spriteBatchAnimLayout        js.Value
	spriteBatchAnimArrayLayout   js.Value
	spriteBatchAnimPipeline      js.Value
	spriteBatchAnimArrayPipeline js.Value
	spritePlaceholder            js.Value
	batchUniformBuffer           js.Value
	batchArena                   uniformArena
	batchArenaAlignment          int
	batchArenaScratch            [spriteBatchAnimUniformSize]byte

	// Retained text batches: registry keyed by game-chosen ID. Content is
	// render-only.
	textBatches map[render.TextBatchID]*gpuTextBatch

	// Retained-batch frame accounting for retainedBatchMetrics:
	// frameRetainedContentBytes is reset once per frame (UpdateResources,
	// the frame boundary every host calls before Render) and grown by every
	// sprite/text batch repack that actually uploads bytes that frame.
	// frameVisibleSprites/Labels and frameRetainedDraws are reset and
	// recomputed by the sprite/text resolution loops in Render.
	frameRetainedContentBytes               uint64
	frameVisibleSprites, frameVisibleLabels int
	frameRetainedDraws                      int

	log         *slog.Logger
	diagnostics diagnostic.Once

	fontProvider *fontatlas.Provider
	textAtlases  map[tess.AtlasKey]gpuTextAtlas

	// texSizes maps TextureID to pixel dimensions for the textured
	// pipeline, populated from the asset manifest in newWebGPU.
	texSizes tess.TexSizes
	tess     tess.Builder

	// textures maps TextureID to its GPU texture/view/bind group, indexed
	// like texSizes (index 0 unused, TextureID 0 means "missing texture").
	textures  []gpuTexture
	particles *gpuParticles
	timing    *gpuTiming // nil when "timestamp-query" was not granted
	drawRuns  []gpuDrawRun

	// sampleCPUTiming mirrors the FrameMetrics gate used for GPU timestamps:
	// without a metrics consumer the per-frame clock reads are pure overhead.
	// cpuTess/cpuUpload/cpuEncode hold the Go-side wall time of the most
	// recent Render call split by phase; cpuTimingValid is false until the
	// first instrumented frame completes.
	sampleCPUTiming               bool
	cpuTess, cpuUpload, cpuEncode time.Duration
	cpuTimingValid                bool
	// cpuResourceUpdate is the most recent UpdateResources call's wall time
	// (texture writes plus any retained sprite batch pack/upload), folded
	// into cpuUpload since it happens before Render's own timing window
	// opens but still uploads GPU buffers this frame.
	cpuResourceUpdate time.Duration

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
	fontProvider, err := fontatlas.New(manifest)
	if err != nil {
		return nil, err
	}
	navigatorGPU := js.Global().Get("navigator").Get("gpu")
	if navigatorGPU.IsUndefined() || navigatorGPU.IsNull() {
		return nil, errWebGPUUnavailable
	}

	adapter, err := awaitPromise(navigatorGPU.Call("requestAdapter"))
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

	format := navigatorGPU.Call("getPreferredCanvasFormat")
	context.Call("configure", map[string]any{
		"device":    device,
		"format":    format,
		"alphaMode": "opaque",
	})

	r := &webgpuRenderer{
		canvas:          canvas,
		clear:           clear,
		manifest:        manifest,
		gpu:             navigatorGPU,
		device:          device,
		queue:           device.Get("queue"),
		context:         context,
		format:          format,
		fontProvider:    fontProvider,
		textAtlases:     make(map[tess.AtlasKey]gpuTextAtlas),
		spriteBatches:   make(map[render.SpriteBatchID]*gpuSpriteBatch),
		textBatches:     make(map[render.TextBatchID]*gpuTextBatch),
		particleScale:   particleScale,
		sampleCPUTiming: wantGPUTiming,
		log:             log,
	}
	r.dev = newJSDevice(r.device, r.queue)
	r.observeDeviceErrors(log)

	r.solidPipeline = r.createSolidPipeline()
	r.textGlyphPipeline = r.createTextGlyphPipeline()
	r.texturedPipeline = r.createTexturedPipeline()
	r.spritePipeline = r.createSpritePipeline()
	r.spriteArrayPipeline = r.createSpriteArrayPipeline()
	r.mipPipeline = r.createMipPipeline()
	r.texSampler = r.createTextureSampler()
	r.mipSampler = r.createMipSampler()
	r.solidBuffer, r.solidCap = r.createVertexBuffer(64 * 1024)
	r.textGlyphBuffer, r.textGlyphCap = r.createSpriteBuffer(64 * 1024)
	r.textLabelBuffer, r.textLabelCap = r.createSpriteBuffer(64 * 1024)
	r.texBuffer, r.texCap = r.createVertexBuffer(64 * 1024)
	r.spriteBuffer, r.spriteCap = r.createSpriteBuffer(64 * 1024)
	r.spriteUniform = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(tess.SpriteUniformSize, gpu.BufferUsageUniform|gpu.BufferUsageCopyDst)))

	r.createSpriteBatchAnimPipelines()
	r.batchArenaAlignment = device.Get("limits").Get("minUniformBufferOffsetAlignment").Int()
	r.batchArena = newUniformArena(spriteBatchAnimUniformSize, r.batchArenaAlignment, 64)
	r.batchUniformBuffer = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(
		r.batchArena.slotSize*r.batchArena.slots, gpu.BufferUsageUniform|gpu.BufferUsageCopyDst)))

	r.textures, r.texSizes = r.buildTextures(manifest)
	r.rebuildSpriteBindGroups()
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

// retainedBatchMetrics reports retained sprite/text batch accounting for the
// frame that just rendered: GPU-resident bytes across every registered
// batch (drawn or not), content bytes actually uploaded this frame, packed
// vs. visible (post-culling) instance/label counts, and the number of
// retained draw calls issued.
func (r *webgpuRenderer) retainedBatchMetrics() (RetainedBatchMetrics, bool) {
	var m RetainedBatchMetrics
	for _, b := range r.spriteBatches {
		m.GPUBytes += uint64(b.residentBytes())
		m.SpritesTotal += b.count
	}
	for _, b := range r.textBatches {
		m.GPUBytes += uint64(b.residentBytes())
		m.LabelsTotal += len(b.labels)
	}
	m.ContentBytes = r.frameRetainedContentBytes
	m.SpritesVisible = r.frameVisibleSprites
	m.LabelsVisible = r.frameVisibleLabels
	m.Draws = r.frameRetainedDraws
	return m, true
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

	var phaseStart float64
	if r.sampleCPUTiming {
		phaseStart = performanceNowMillis()
	}

	r.tess.SetSpriteArray(r.spriteArrayEnabled)
	textBatchIDs := collectTextBatchIDs(frame.Commands)
	r.prepTextBatches(textBatchIDs, frame.Camera, w, h)
	runs := r.tess.Build(tess.Frame(frame), w, h, r.fontProvider, r.texSizes)

	// tess ends here: command building/packing and atlas-coverage growth are
	// done. Retained-batch repack (finalizeTextBatches, the sprite-batch
	// resolution below) does its own packing and queue.writeBuffer calls, so
	// it counts toward upload like every other buffer write in this
	// function — never hidden outside the timed window.
	var buildEnd float64
	if r.sampleCPUTiming {
		buildEnd = performanceNowMillis()
	}

	r.finalizeTextBatches(textBatchIDs, frame.Camera, w, h)
	solidBytes, texBytes := r.tess.Streams()
	spriteBytes := r.tess.SpriteStream()
	textGlyphBytes := r.tess.TextGlyphStream()
	textLabelBytes := r.tess.TextLabelStream()
	r.drawRuns = r.drawRuns[:0]
	drawRuns := r.drawRuns
	for _, run := range runs {
		switch run.Kind {
		case tess.DrawRunSolid:
			if run.VertexCount == 0 {
				continue
			}
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:   tess.DrawRunSolid,
				offset: run.ByteOffset / tess.SolidVertexStride,
				count:  run.VertexCount,
			})
		case tess.DrawRunTextured:
			if run.VertexCount == 0 {
				continue
			}
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:    tess.DrawRunTextured,
				texture: run.Texture,
				offset:  run.ByteOffset / tess.TexVertexStride,
				count:   run.VertexCount,
			})
		case tess.DrawRunSprites:
			if run.VertexCount == 0 {
				continue
			}
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:    tess.DrawRunSprites,
				texture: run.Texture,
				offset:  run.ByteOffset / tess.SpriteStride,
				count:   run.VertexCount,
			})
		case tess.DrawRunText:
			if run.VertexCount == 0 {
				continue
			}
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:     tess.DrawRunText,
				atlasKey: run.AtlasKey,
				offset:   run.ByteOffset / tess.TextGlyphStride,
				count:    run.VertexCount,
			})
		case tess.DrawRunParticles:
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:      tess.DrawRunParticles,
				particles: run.Particles,
			})
		case tess.DrawRunSpriteBatch:
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:        tess.DrawRunSpriteBatch,
				spriteBatch: run.SpriteBatch,
				batchSlot:   -1,
			})
		case tess.DrawRunTextBatch:
			drawRuns = append(drawRuns, gpuDrawRun{
				kind:      tess.DrawRunTextBatch,
				textBatch: run.TextBatch,
			})
		}
	}
	r.drawRuns = drawRuns

	// Resolve retained sprite batch commands before any upload: registry
	// lookup (missing batches warn once and are skipped), whole-batch culling
	// against the animation-inflated bounds, and one arena slot of animation
	// uniforms per animated command. Two draws of the same batch always get
	// two distinct slots, since queue.writeBuffer lands before the command
	// buffer and a shared slot would show both draws the last packed value.
	hasSpriteBatchDraw := false
	r.frameVisibleSprites = 0
	r.frameRetainedDraws = 0
	animatedBatchDraws := 0
	for i := range drawRuns {
		if drawRuns[i].kind != tess.DrawRunSpriteBatch {
			continue
		}
		cmd := drawRuns[i].spriteBatch
		batch, ok := r.spriteBatches[cmd.Batch]
		if !ok {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "webgpu-sprite-batch-missing", ID: uint64(cmd.Batch)},
				"webgpu: DrawSpriteBatch references a missing or deleted batch; command skipped")
			continue
		}
		if batch.count == 0 {
			continue
		}
		canCull := !batch.hasRotation || renderbatch.WorldBoundsPreserveScreenRotation(frame.Camera)
		if view, ok := webgpuBatchView(frame.Camera, cmd.Clip); ok && canCull {
			bounds, boundsOK := spriteBatchInflatedBounds(batch.bounds, batch.hasBounds, batch.maxBounce, cmd.Animation)
			if !boundsOK || !renderbatch.IntersectsView(bounds, view) {
				continue
			}
		}
		if batch.animated {
			animatedBatchDraws++
		}
		drawRuns[i].batch = batch
		hasSpriteBatchDraw = true
		r.frameVisibleSprites += batch.count
		r.frameRetainedDraws += len(batch.ranges)
	}
	r.ensureBatchUniformArena(animatedBatchDraws)
	r.batchArena.reset()
	for i := range drawRuns {
		batch := drawRuns[i].batch
		if drawRuns[i].kind != tess.DrawRunSpriteBatch || batch == nil || !batch.animated {
			continue
		}
		offset, ok := r.batchArena.alloc()
		if !ok {
			panic("wasm: retained sprite uniform arena capacity miscalculated")
		}
		packSpriteBatchAnimUniforms(r.batchArenaScratch[:], frame.Camera, w, h, drawRuns[i].spriteBatch.Animation)
		r.queue.Call("writeBuffer", r.batchUniformBuffer, offset, bytesToJS(r.batchArenaScratch[:]))
		drawRuns[i].batchSlot = offset
	}

	// Resolve retained text batch commands the same way: registry lookup
	// (missing batches warn once and are skipped) and whole-batch culling
	// against the batch's screen-space bounds. finalizeTextBatches already
	// repacked any batch that changed, so this only reads the result.
	hasTextBatchDraw := r.resolveTextBatchDraws(drawRuns, frame.Camera, w, h)

	r.ensureSolidBuffer(len(solidBytes))
	r.ensureTextGlyphBuffer(len(textGlyphBytes))
	r.ensureTextLabelBuffer(len(textLabelBytes))

	if len(solidBytes) > 0 {
		r.queue.Call("writeBuffer", r.solidBuffer, 0, bytesToJS(solidBytes))
	}
	if len(textGlyphBytes) > 0 {
		r.queue.Call("writeBuffer", r.textGlyphBuffer, 0, bytesToJS(textGlyphBytes))
	}
	if len(textLabelBytes) > 0 {
		r.queue.Call("writeBuffer", r.textLabelBuffer, 0, bytesToJS(textLabelBytes))
	}

	r.ensureTexBuffer(len(texBytes))
	if len(texBytes) > 0 {
		r.queue.Call("writeBuffer", r.texBuffer, 0, bytesToJS(texBytes))
	}
	r.ensureSpriteBuffer(len(spriteBytes))
	if len(spriteBytes) > 0 {
		r.queue.Call("writeBuffer", r.spriteBuffer, 0, bytesToJS(spriteBytes))
	}
	if len(spriteBytes) > 0 || len(textGlyphBytes) > 0 || hasSpriteBatchDraw || hasTextBatchDraw {
		tess.PackSpriteUniforms(r.spriteUniformScratch[:], frame.Camera, w, h)
		r.queue.Call("writeBuffer", r.spriteUniform, 0, bytesToJS(r.spriteUniformScratch[:]))
	}

	var uploadEnd float64
	if r.sampleCPUTiming {
		uploadEnd = performanceNowMillis()
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
	// The render pass is timed whenever a metrics consumer exists, with or
	// without particles; timingWrote tells the readback whether the particle
	// compute queries were also written this frame (stale otherwise).
	sampleTiming := r.timing != nil

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

	// Runs are submitted in painter order. Mesh pipelines read packed vertex
	// buffers; sprite/text pipelines use firstVertex to index shared storage
	// records without relying on firstInstance.
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
		case tess.DrawRunSprites:
			if r.spriteArrayEnabled {
				pass.Call("setPipeline", r.spriteArrayPipeline)
				pass.Call("setBindGroup", 0, r.spriteArrayBindGroup)
			} else {
				gt, ok := r.textureFor(run.texture)
				if !ok || gt.spriteBindGroup.IsUndefined() || gt.spriteBindGroup.IsNull() {
					continue
				}
				pass.Call("setPipeline", r.spritePipeline)
				pass.Call("setBindGroup", 0, gt.spriteBindGroup)
			}
			pass.Call("draw", run.count*6, 1, run.offset*6, 0)
		case tess.DrawRunText:
			atlas, ok := r.textAtlasFor(run.atlasKey)
			if !ok {
				continue
			}
			pass.Call("setPipeline", r.textGlyphPipeline)
			pass.Call("setBindGroup", 0, atlas.glyphBindGroup)
			pass.Call("draw", run.count*6, 1, run.offset*6, 0)
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

	pass.Call("end")
	timingSlot := -1
	if sampleTiming {
		timingSlot = r.timing.encodeResolve(encoder, r.particles.timingWrote)
	}
	commandBuffer := encoder.Call("finish")
	r.queue.Call("submit", []any{commandBuffer})
	if timingSlot >= 0 {
		r.timing.readback(timingSlot)
	}
	r.particles.endFrame()
	if r.sampleCPUTiming {
		encodeEnd := performanceNowMillis()
		r.cpuTess = durationFromMillis(buildEnd - phaseStart)
		r.cpuUpload = durationFromMillis(uploadEnd-buildEnd) + r.cpuResourceUpdate
		r.cpuEncode = durationFromMillis(encodeEnd - uploadEnd)
		r.cpuTimingValid = true
	}
	r.signalReadyAfterPresent()
}

// renderCPUTimings reports the Go-side wall time of the most recent Render
// call, split into stream construction (Build + run mapping), buffer upload
// (JS heap copy + writeBuffer) and command encoding (render passes + submit). ok is
// false until the first instrumented frame, or when no metrics consumer
// enabled sampling.
func (r *webgpuRenderer) renderCPUTimings() (tessTime, upload, encode time.Duration, ok bool) {
	if !r.cpuTimingValid {
		return 0, 0, 0, false
	}
	return r.cpuTess, r.cpuUpload, r.cpuEncode, true
}

// particleGPUTimings reports the most recent timestamp-query sample: the
// particle prepare pass (kick + emit), the simulate pass (including the pass
// boundary before it) and the main render pass. Frames without particle
// commands still sample the render pass and report zero for the compute
// stages. ok is false until the first sample resolves, or always on adapters
// without "timestamp-query".
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
	module := r.device.Call("createShaderModule", map[string]any{"code": shaders.ParticlesCompositeWGSL})
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
			r.canvas.Call("setAttribute", "data-arasy-ready", "webgpu")
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
	// spriteBatch is the retained batch command; batch is its resolved
	// registry entry (nil when missing, deleted, empty or culled) and
	// batchSlot the animation-uniform arena offset for animated draws.
	spriteBatch render.DrawSpriteBatch
	batch       *gpuSpriteBatch
	batchSlot   int
	// textBatch is the retained text batch command; text is its resolved
	// registry entry (nil when missing, deleted, empty or culled).
	textBatch render.DrawTextBatch
	text      *gpuTextBatch
}

// webgpuBatchView returns the world-space rect a retained batch must
// intersect to produce pixels: the camera's resolved visible world,
// tightened by the command clip when set. ok is false when the camera has no
// usable world area, in which case the caller draws without culling.
func webgpuBatchView(camera render.Camera, clip math2d.Rect) (math2d.Rect, bool) {
	view := camera.Resolve().World
	if view.Width() <= 0 || view.Height() <= 0 {
		return math2d.Rect{}, false
	}
	if clip != (math2d.Rect{}) {
		view.Min.X = max(view.Min.X, clip.Min.X)
		view.Min.Y = max(view.Min.Y, clip.Min.Y)
		view.Max.X = min(view.Max.X, clip.Max.X)
		view.Max.Y = min(view.Max.Y, clip.Max.Y)
		// An empty intersection still culls: IntersectsView requires a
		// positive-area overlap, and a degenerate rect overlaps nothing.
	}
	return view, true
}

// particleTarget is one pooled offscreen target plus its composite bind group.
type particleTarget struct {
	texture   js.Value
	view      js.Value
	bindGroup js.Value
}

func (r *webgpuRenderer) createSolidPipeline() js.Value {
	module := r.dev.CreateShaderModule(shaders.SolidWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.SolidPipelineDesc(module, r.format.String(), msaaSampleCount)))
}

func (r *webgpuRenderer) createTextGlyphPipeline() js.Value {
	module := r.dev.CreateShaderModule(shaders.TextGlyphWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.TextGlyphPipelineDesc(module, r.format.String(), msaaSampleCount)))
}

func (r *webgpuRenderer) createTexturedPipeline() js.Value {
	module := r.dev.CreateShaderModule(shaders.TexturedWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.TexturedPipelineDesc(module, r.format.String(), msaaSampleCount)))
}

func (r *webgpuRenderer) createSpritePipeline() js.Value {
	module := r.dev.CreateShaderModule(shaders.SpriteWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.SpritePipelineDesc(module, r.format.String(), msaaSampleCount)))
}

func (r *webgpuRenderer) createSpriteArrayPipeline() js.Value {
	module := r.dev.CreateShaderModule(shaders.SpriteArrayWGSL)
	return jsHandle(r.dev.CreateRenderPipeline(gpu.SpritePipelineDesc(module, r.format.String(), msaaSampleCount)))
}

// createMipPipeline builds the internal pipeline used by generateMipmaps: a
// fullscreen triangle (no vertex buffer) sampling one mip level into the
// next, one render pass per level.
func (r *webgpuRenderer) createMipPipeline() js.Value {
	module := r.dev.CreateShaderModule(shaders.MipmapWGSL)
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
	atlas          *tess.Atlas
	texture        js.Value
	metricBuffer   js.Value
	view           js.Value
	sampler        js.Value
	glyphBindGroup js.Value
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
		cached.metricBuffer.Call("destroy")
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
	samplerDesc := gpu.NearestSamplerDesc()
	if atlas.LinearFilter {
		samplerDesc = gpu.LinearSamplerDesc()
	}
	sampler := r.dev.CreateSampler(samplerDesc)
	viewValue := jsHandle(view)
	samplerValue := jsHandle(sampler)
	metricBytes := atlas.GlyphMetricStream()
	metricBuffer := jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(len(metricBytes), gpu.BufferUsageStorage|gpu.BufferUsageCopyDst)))
	r.queue.Call("writeBuffer", metricBuffer, 0, bytesToJS(metricBytes))
	glyphBindGroup := r.createTextGlyphBindGroup(viewValue, samplerValue, metricBuffer, r.textGlyphBuffer, r.textLabelBuffer)
	return gpuTextAtlas{atlas: atlas, texture: jsHandle(texture), metricBuffer: metricBuffer, view: viewValue, sampler: samplerValue, glyphBindGroup: glyphBindGroup}
}

func (r *webgpuRenderer) createVertexBuffer(size int) (js.Value, int) {
	return jsHandle(r.dev.CreateBuffer(gpu.VertexBufferDesc(size))), size
}

func (r *webgpuRenderer) createSpriteBuffer(size int) (js.Value, int) {
	return jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(size, gpu.BufferUsageStorage|gpu.BufferUsageCopyDst))), size
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

func (r *webgpuRenderer) ensureTextGlyphBuffer(size int) {
	if size <= r.textGlyphCap {
		return
	}
	newCap := growBufferCap(r.textGlyphCap, size)
	r.textGlyphBuffer, r.textGlyphCap = r.createSpriteBuffer(newCap)
	r.rebuildTextGlyphBindGroups()
}

func (r *webgpuRenderer) ensureTextLabelBuffer(size int) {
	if size <= r.textLabelCap {
		return
	}
	newCap := growBufferCap(r.textLabelCap, size)
	r.textLabelBuffer, r.textLabelCap = r.createSpriteBuffer(newCap)
	r.rebuildTextGlyphBindGroups()
}

// createTextGlyphBindGroup builds a text-pipeline bind group against
// explicit glyph/label buffers: the shared streaming buffers for
// DrawText, or one retained batch's own persistent buffers.
func (r *webgpuRenderer) createTextGlyphBindGroup(view, sampler, metrics, glyphBuffer, labelBuffer js.Value) js.Value {
	layout := r.textGlyphPipeline.Call("getBindGroupLayout", 0)
	return r.device.Call("createBindGroup", map[string]any{
		"layout": layout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": view},
			map[string]any{"binding": 1, "resource": sampler},
			map[string]any{"binding": 2, "resource": map[string]any{"buffer": glyphBuffer}},
			map[string]any{"binding": 3, "resource": map[string]any{"buffer": labelBuffer}},
			map[string]any{"binding": 4, "resource": map[string]any{"buffer": metrics}},
			map[string]any{"binding": 5, "resource": map[string]any{"buffer": r.spriteUniform}},
		},
	})
}

// rebuildTextGlyphBindGroups rebuilds every atlas's streaming bind group
// after the shared glyph/label buffers grow. Retained text batches keep
// their own persistent buffers and are unaffected; their bind groups are
// rebuilt by repackTextBatch whenever content, projection, or atlas
// identity actually changes.
func (r *webgpuRenderer) rebuildTextGlyphBindGroups() {
	for key, atlas := range r.textAtlases {
		atlas.glyphBindGroup = r.createTextGlyphBindGroup(atlas.view, atlas.sampler, atlas.metricBuffer, r.textGlyphBuffer, r.textLabelBuffer)
		r.textAtlases[key] = atlas
	}
}

func (r *webgpuRenderer) ensureTexBuffer(size int) {
	if size <= r.texCap {
		return
	}

	newCap := growBufferCap(r.texCap, size)
	r.texBuffer, r.texCap = r.createVertexBuffer(newCap)
}

func (r *webgpuRenderer) ensureSpriteBuffer(size int) {
	if size <= r.spriteCap {
		return
	}

	newCap := growBufferCap(r.spriteCap, size)
	r.spriteBuffer, r.spriteCap = r.createSpriteBuffer(newCap)
	r.rebuildSpriteBindGroups()
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

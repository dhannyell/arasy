//go:build js && wasm

package wasm

import (
	"encoding/binary"
	"syscall/js"

	"github.com/dhannyell/arasy/internal/render/gpu/particlegpu"
	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/platform/wasm/gpu"
	"github.com/dhannyell/arasy/render"
)

type gpuEmitter struct {
	decl     particle.EffectDecl
	seed     uint64
	emission particle.EmissionState
	capacity int
	lastSeen uint64
	// missingTexture selects the untinted magenta placeholder pipeline.
	missingTexture bool

	hot          js.Value // capacity × 24 B, the simulated state
	cold         js.Value // capacity × 32 B, written once at spawn
	alive        js.Value // survivor list, rebuilt by every simulated frame
	dead         js.Value // free-slot stack, persists across frames
	counters     js.Value // {dead, alive, pad, pad} atomics
	drawIndirect js.Value // STORAGE in compute; INDIRECT in render pass
	uniforms     js.Value

	computeBind js.Value // emit + simulate share one layout and group
	renderBind  js.Value
	textureBind js.Value
	scratch     [particlegpu.UniformSize]byte
}

type gpuParticles struct {
	device, queue js.Value
	renderer      *webgpuRenderer

	emitPipeline     js.Value
	simulatePipeline js.Value
	drawAlpha        js.Value // premultiplied source-over, MSAA 4x
	drawAdditive     js.Value // additive blending, MSAA 4x
	drawMissing      js.Value // opaque magenta, deliberately ignores tint

	// Single-sample variants for the particle offscreen target, built only
	// when Config.ParticleOffscreenScale enables offscreen compositing. The
	// additive variant keeps destination alpha untouched so the source-over
	// composite of the offscreen stays mathematically exact for both blends.
	drawAlphaOff    js.Value
	drawAdditiveOff js.Value
	drawMissingOff  js.Value

	computeLayout js.Value
	renderLayout  js.Value
	textureLayout js.Value

	whiteTexture   js.Value // 1×1 bind target for untextured effects
	missingTexture js.Value // magenta 1×1 for a declared texture that failed
	sampler        js.Value

	emitters map[particlegpu.EmitterKey]*gpuEmitter
	frame    uint64

	// timing instrumentation: armTiming runs once per frame before the
	// encodeSim loop, and the first compute pass created afterwards carries
	// the frame-begin timestamp. timingWrote tells the renderer whether that
	// pass actually existed (a fully paused frame encodes none).
	timing      *gpuTiming
	timingArmed bool
	timingWrote bool
}

type gpuParticleRun struct {
	command render.DrawParticles
	camera  render.Camera
}

// newGpuParticles builds the shared particle GPU state once: bind group
// layouts, the three compute pipelines, the render blend variants (plus
// their single-sample offscreen twins when withOffscreen is set), the white
// 1×1 texture for untextured effects and the sampler.
func newGpuParticles(r *webgpuRenderer, withOffscreen bool) *gpuParticles {
	g := &gpuParticles{
		device:   r.device,
		queue:    r.queue,
		renderer: r,
		emitters: make(map[particlegpu.EmitterKey]*gpuEmitter),
	}
	g.computeLayout = g.computeBindLayout()
	g.renderLayout = g.renderBindLayout()
	g.textureLayout = g.textureBindLayout()
	g.whiteTexture = g.createSolidTexture([4]byte{255, 255, 255, 255})
	g.missingTexture = g.createSolidTexture([4]byte{255, 0, 255, 255})
	g.sampler = g.device.Call("createSampler", map[string]any{
		"magFilter": "linear",
		"minFilter": "linear",
	})
	g.buildPipelines(r.format, withOffscreen, r.msaaSamples)
	return g
}

// computeBindLayout is shared by emit and simulate. With the simulate
// dispatch sized on the CPU there is no dispatch-indirect buffer, so no
// usage-scope conflict forces separate layouts anymore.
func (g *gpuParticles) computeBindLayout() js.Value {
	compute := js.Global().Get("GPUShaderStage").Get("COMPUTE").Int()
	storage := map[string]any{"type": "storage"}
	entry := func(binding int, buffer map[string]any) map[string]any {
		return map[string]any{"binding": binding, "visibility": compute, "buffer": buffer}
	}
	return g.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			entry(0, storage), // hot particle pool
			entry(1, storage), // survivor list
			entry(2, storage), // dead list
			entry(3, storage), // counters
			entry(4, storage), // draw indirect args
			entry(5, map[string]any{"type": "uniform"}),
			entry(6, storage), // cold particle pool (spawn-time state)
		},
	})
}

// renderBindLayout feeds the vertex-pulling shader: uniforms, hot/cold pools
// and the survivors list, all read-only (vertex stages cannot write storage).
func (g *gpuParticles) renderBindLayout() js.Value {
	vertex := js.Global().Get("GPUShaderStage").Get("VERTEX").Int()
	readOnly := map[string]any{"type": "read-only-storage"}
	return g.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			map[string]any{"binding": 0, "visibility": vertex, "buffer": map[string]any{"type": "uniform"}},
			map[string]any{"binding": 1, "visibility": vertex, "buffer": readOnly},
			map[string]any{"binding": 2, "visibility": vertex, "buffer": readOnly},
			map[string]any{"binding": 3, "visibility": vertex, "buffer": readOnly},
		},
	})
}

func (g *gpuParticles) textureBindLayout() js.Value {
	fragment := js.Global().Get("GPUShaderStage").Get("FRAGMENT").Int()
	return g.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			map[string]any{"binding": 0, "visibility": fragment, "texture": map[string]any{}},
			map[string]any{"binding": 1, "visibility": fragment, "sampler": map[string]any{}},
		},
	})
}

func (g *gpuParticles) buildPipelines(format js.Value, withOffscreen bool, samples int) {
	computeModule := g.device.Call("createShaderModule", map[string]any{"code": shaders.ParticlesComputeWGSL})
	computePipelineLayout := g.device.Call("createPipelineLayout", map[string]any{
		"bindGroupLayouts": []any{g.computeLayout},
	})
	computePipeline := func(entry string) js.Value {
		return g.device.Call("createComputePipeline", map[string]any{
			"layout":  computePipelineLayout,
			"compute": map[string]any{"module": computeModule, "entryPoint": entry},
		})
	}
	g.emitPipeline = computePipeline("emit")
	g.simulatePipeline = computePipeline("simulate")

	renderModule := g.device.Call("createShaderModule", map[string]any{"code": shaders.ParticlesRenderWGSL})
	renderLayout := g.device.Call("createPipelineLayout", map[string]any{
		"bindGroupLayouts": []any{g.renderLayout, g.textureLayout},
	})
	pipeline := func(fragment string, blend map[string]any, samples int) js.Value {
		return g.device.Call("createRenderPipeline", map[string]any{
			"layout": renderLayout,
			"vertex": map[string]any{"module": renderModule, "entryPoint": "vs_main"},
			"fragment": map[string]any{
				"module":     renderModule,
				"entryPoint": fragment,
				"targets":    []any{map[string]any{"format": format, "blend": blend}},
			},
			"primitive":   map[string]any{"topology": "triangle-list"},
			"multisample": map[string]any{"count": samples},
		})
	}
	g.drawAlpha = pipeline("fs_main", gpu.PremultipliedBlend(), samples)
	g.drawAdditive = pipeline("fs_main", additiveBlend(), samples)
	g.drawMissing = pipeline("fs_missing", gpu.PremultipliedBlend(), samples)
	if withOffscreen {
		g.drawAlphaOff = pipeline("fs_main", gpu.PremultipliedBlend(), 1)
		g.drawAdditiveOff = pipeline("fs_main", additiveOffscreenBlend(), 1)
		g.drawMissingOff = pipeline("fs_missing", gpu.PremultipliedBlend(), 1)
	}
}

func (g *gpuParticles) createSolidTexture(color [4]byte) js.Value {
	texture := g.device.Call("createTexture", map[string]any{
		"size":   []any{1, 1},
		"format": "rgba8unorm",
		"usage":  gpu.TextureUsageTextureBinding | gpu.TextureUsageCopyDst,
	})
	g.queue.Call("writeTexture",
		map[string]any{"texture": texture},
		bytesToJS(color[:]),
		map[string]any{"bytesPerRow": 4},
		map[string]any{"width": 1, "height": 1},
	)
	return texture
}

// additiveBlend is the premultiplied additive variant: source adds onto the
// destination without attenuating it (fire, sparks, glow).
func additiveBlend() map[string]any {
	factor := map[string]any{"srcFactor": "one", "dstFactor": "one", "operation": "add"}
	return map[string]any{"color": factor, "alpha": factor}
}

// additiveOffscreenBlend accumulates additive color into the offscreen while
// leaving its alpha untouched. The composite is source-over — dst*(1-a) + rgb
// — so additive light must carry zero coverage, or it would wrongly attenuate
// whatever is behind the composite.
func additiveOffscreenBlend() map[string]any {
	return map[string]any{
		"color": map[string]any{"srcFactor": "one", "dstFactor": "one", "operation": "add"},
		"alpha": map[string]any{"srcFactor": "zero", "dstFactor": "one", "operation": "add"},
	}
}

func (g *gpuParticles) newEmitter(decl particle.EffectDecl, seed uint64) *gpuEmitter {
	e := &gpuEmitter{decl: decl, seed: seed, capacity: decl.Capacity}
	usage := js.Global().Get("GPUBufferUsage")
	storage := usage.Get("STORAGE").Int() | usage.Get("COPY_DST").Int()

	e.hot = g.buffer(decl.Capacity*particlegpu.HotStride, storage)
	e.cold = g.buffer(decl.Capacity*particlegpu.ColdStride, storage)
	e.alive = g.buffer(decl.Capacity*4, storage)
	e.dead = g.buffer(decl.Capacity*4, storage)
	e.counters = g.buffer(16, storage)
	e.drawIndirect = g.buffer(particlegpu.DrawArgsSize, storage|usage.Get("INDIRECT").Int())
	e.uniforms = g.buffer(particlegpu.UniformSize, usage.Get("UNIFORM").Int()|usage.Get("COPY_DST").Int())

	// Dead list starts full (every slot free), counters at {capacity, 0} and
	// draw args at {3,0,0,0}: three vertices per instance, zero instances
	// until simulate runs. The zero-initialized hot pool marks every slot
	// dead (life == 0) for the sequential simulate scan.
	init := make([]byte, decl.Capacity*4)
	for i := 0; i < decl.Capacity; i++ {
		binary.LittleEndian.PutUint32(init[i*4:], uint32(i))
	}
	g.queue.Call("writeBuffer", e.dead, 0, bytesToJS(init))
	g.queue.Call("writeBuffer", e.counters, 0, bytesToJS(packU32(uint32(decl.Capacity), 0, 0, 0)))
	g.queue.Call("writeBuffer", e.drawIndirect, 0, bytesToJS(packU32(3, 0, 0, 0)))

	e.computeBind = g.bindCompute(e)
	e.renderBind = g.bindRender(e)
	e.textureBind, e.missingTexture = g.bindTexture(decl) // white 1×1 when untextured
	return e
}

// buffer creates one GPU buffer with the given byte size and usage bits.
func (g *gpuParticles) buffer(size, usage int) js.Value {
	return g.device.Call("createBuffer", map[string]any{
		"size":  size,
		"usage": usage,
	})
}

// bindCompute builds the emit/simulate bind group. Binding numbers mirror
// the @group(0) declarations in particle_compute.wgsl.
func (g *gpuParticles) bindCompute(e *gpuEmitter) js.Value {
	return g.device.Call("createBindGroup", map[string]any{
		"layout": g.computeLayout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": map[string]any{"buffer": e.hot}},
			map[string]any{"binding": 1, "resource": map[string]any{"buffer": e.alive}},
			map[string]any{"binding": 2, "resource": map[string]any{"buffer": e.dead}},
			map[string]any{"binding": 3, "resource": map[string]any{"buffer": e.counters}},
			map[string]any{"binding": 4, "resource": map[string]any{"buffer": e.drawIndirect}},
			map[string]any{"binding": 5, "resource": map[string]any{"buffer": e.uniforms}},
			map[string]any{"binding": 6, "resource": map[string]any{"buffer": e.cold}},
		},
	})
}

// packU32 serializes u32 values in little-endian order, the layout GPU
// buffers expect, for the small init writes (counters, indirect args).
func packU32(values ...uint32) []byte {
	out := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[i*4:], v)
	}
	return out
}

// bindRender builds the vertex-pulling bind group: the vertex shader indexes
// the survivor list by instance_index. Storage buffers are read-only here:
// core WebGPU forbids writable storage in vertex stages.
func (g *gpuParticles) bindRender(e *gpuEmitter) js.Value {
	return g.device.Call("createBindGroup", map[string]any{
		"layout": g.renderLayout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": map[string]any{"buffer": e.uniforms}},
			map[string]any{"binding": 1, "resource": map[string]any{"buffer": e.hot}},
			map[string]any{"binding": 2, "resource": map[string]any{"buffer": e.alive}},
			map[string]any{"binding": 3, "resource": map[string]any{"buffer": e.cold}},
		},
	})
}

// bindTexture resolves the effect's texture bind group once per emitter.
// An untextured effect binds the shared white 1×1 so the fragment shader
// needs no branch: sampling white leaves the interpolated color as-is.
func (g *gpuParticles) bindTexture(decl particle.EffectDecl) (js.Value, bool) {
	view := g.whiteTexture.Call("createView")
	missing := false
	if decl.Texture != "" {
		missing = true
		view = g.missingTexture.Call("createView")
		if gt, ok := g.renderer.textureFor(g.renderer.manifest.Texture(decl.Texture)); ok {
			missing = false
			// gt.texture may be the shared array resource. gt.view is always
			// the declared texture's single-layer 2D view, which matches the
			// particle texture layout and also works for standalone textures.
			view = gt.view
		}
	}
	bindGroup := g.device.Call("createBindGroup", map[string]any{
		"layout": g.textureLayout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": view},
			map[string]any{"binding": 1, "resource": g.sampler},
		},
	})
	return bindGroup, missing
}

// armTiming resets the per-frame timing state so this frame's compute passes
// carry their timestamps.
func (g *gpuParticles) armTiming(t *gpuTiming) {
	g.timing = t
	g.timingArmed = true
	g.timingWrote = false
}

// simRequest is one emitter whose dispatches encodeSims must record this
// frame, produced by prepareEmitter.
type simRequest struct {
	e         *gpuEmitter
	emitCount int
}

// encodeSims runs the CPU-side accounting for every particle command in
// painter order, then records all emit dispatches in one shared compute pass
// and all simulate dispatches in a second. Emitter buffers are disjoint, so
// dispatches from different emitters need no ordering between them.
//
// There is no kick stage: prepareEmitter resets the alive cursor and the
// indirect instance count with two 4-byte queue writes, which execute before
// any pass in the submission. The simulate dispatch covers the whole pool
// (sequential slot scan), so its size is known on the CPU and the old
// dispatch-indirect buffer is gone. The pass boundary between emit and
// simulate makes freshly spawned particles visible to the same-frame scan.
func (g *gpuParticles) encodeSims(
	encoder js.Value,
	runs []gpuParticleRun,
	w, h int,
) {
	sims := make([]simRequest, 0, len(runs))
	for _, run := range runs {
		if e, emitCount, ok := g.prepareEmitter(run.command, run.camera, w, h); ok {
			sims = append(sims, simRequest{e: e, emitCount: emitCount})
		}
	}
	if len(sims) == 0 {
		return
	}

	sampled := g.timingArmed
	if sampled {
		g.timingArmed = false
		g.timingWrote = true
	}

	preparePass := beginComputePass(encoder, sampled, g.timing.prepareTimestampWrites)
	emitPipelineSet := false
	for _, sim := range sims {
		if sim.emitCount == 0 {
			continue
		}
		if !emitPipelineSet {
			preparePass.Call("setPipeline", g.emitPipeline)
			emitPipelineSet = true
		}
		preparePass.Call("setBindGroup", 0, sim.e.computeBind)
		preparePass.Call("dispatchWorkgroups", (sim.emitCount+particlegpu.Workgroup-1)/particlegpu.Workgroup)
	}
	preparePass.Call("end")

	simulatePass := beginComputePass(encoder, sampled, g.timing.simulateTimestampWrites)
	simulatePass.Call("setPipeline", g.simulatePipeline)
	for _, sim := range sims {
		simulatePass.Call("setBindGroup", 0, sim.e.computeBind)
		simulatePass.Call("dispatchWorkgroups", (sim.e.capacity+particlegpu.Workgroup-1)/particlegpu.Workgroup)
	}
	simulatePass.Call("end")
}

// beginComputePass opens a compute pass, attaching the given timestampWrites
// only on frames being sampled (timed adapters, one sample per frame).
func beginComputePass(encoder js.Value, sampled bool, writes func() map[string]any) js.Value {
	if !sampled {
		return encoder.Call("beginComputePass")
	}
	return encoder.Call("beginComputePass", map[string]any{"timestampWrites": writes()})
}

// prepareEmitter updates one emitter's CPU accounting and uploads its
// uniforms, returning the emitter and this frame's emit count when its
// dispatches should be encoded. ok is false for duplicate commands, unknown
// effects and paused frames (dt <= 0), which freeze the particle buffers —
// including the survivor list and draw args — matching particle.Emitter.Step.
func (g *gpuParticles) prepareEmitter(
	cmd render.DrawParticles,
	camera render.Camera,
	w, h int,
) (*gpuEmitter, int, bool) {
	decl, ok := g.renderer.manifest.EffectDecl(cmd.Effect)
	if !ok {
		return nil, 0, false
	}

	key := particlegpu.EmitterKey{Effect: cmd.Effect, Emitter: cmd.Emitter}
	e, exists := g.emitters[key]
	// The first command for one emitter key owns simulation this frame. In
	// particular, never destroy/recreate buffers for a later command with a
	// different seed after this frame's accounting already claimed the old
	// ones for the encoder.
	if exists && e.lastSeen == g.frame {
		return nil, 0, false
	}
	if !exists || e.seed != cmd.Seed {
		if exists {
			e.destroy()
		}
		e = g.newEmitter(decl, cmd.Seed)
		g.emitters[key] = e
	}
	e.lastSeen = g.frame

	frame := e.emission.Advance(
		cmd.Pos,
		decl.EmitRate,
		cmd.Intensity,
		decl.BurstSize,
		cmd.Burst,
		cmd.Dt,
	)
	emitCount := max(frame.Count, 0)
	particlegpu.PackUniforms(e.scratch[:], e.decl, cmd, frame, emitCount, e.seed, e.capacity, camera, w, h)
	g.queue.Call("writeBuffer", e.uniforms, 0, bytesToJS(e.scratch[:]))

	if cmd.Dt <= 0 {
		return nil, 0, false
	}

	// Queue writes land before any pass in this frame's submission: reset the
	// survivor-list cursor and the indirect instance count so the simulate
	// scan rebuilds both. Skipped on paused frames, which must keep them.
	g.queue.Call("writeBuffer", e.counters, particlegpu.CountersAliveOffset, bytesToJS(packU32(0)))
	g.queue.Call("writeBuffer", e.drawIndirect, particlegpu.DrawInstanceOffset, bytesToJS(packU32(0)))

	// The GPU consumes the full requested hash range even when its dead stack
	// is exhausted and some emit invocations drop their spawn.
	e.emission.Emitted += uint32(emitCount)
	return e, emitCount, true
}

// draw submits one emitter at its exact painter-order position in the main
// MSAA pass. The indirect instance count was produced by simulate, so no GPU
// readback is needed.
func (g *gpuParticles) draw(
	pass js.Value,
	cmd render.DrawParticles,
	camera render.Camera,
	w, h int,
) {
	g.drawTo(pass, cmd, camera, 1, w, h, false)
}

// drawTo submits one emitter into either the main MSAA pass (offscreen
// false, clipScale 1, target dims = canvas dims) or a single-sample particle
// offscreen target whose dimensions are the canvas dims times clipScale. The
// world-to-clip transform is resolution-independent, so only the scissor
// needs the scale.
func (g *gpuParticles) drawTo(
	pass js.Value,
	cmd render.DrawParticles,
	camera render.Camera,
	clipScale float64,
	targetW, targetH int,
	offscreen bool,
) {
	e := g.emitters[particlegpu.EmitterKey{Effect: cmd.Effect, Emitter: cmd.Emitter}]
	if e == nil || e.seed != cmd.Seed || e.lastSeen != g.frame {
		return
	}

	pipeline := g.drawAlpha
	if offscreen {
		pipeline = g.drawAlphaOff
		if e.missingTexture {
			pipeline = g.drawMissingOff
		} else if e.decl.Blend == particle.BlendAdditive {
			pipeline = g.drawAdditiveOff
		}
	} else if e.missingTexture {
		pipeline = g.drawMissing
	} else if e.decl.Blend == particle.BlendAdditive {
		pipeline = g.drawAdditive
	}
	pass.Call("setPipeline", pipeline)
	pass.Call("setBindGroup", 0, e.renderBind)
	pass.Call("setBindGroup", 1, e.textureBind)

	hasClip := cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0
	if !applyClipScissor(pass, cmd.Clip, camera, clipScale, targetW, targetH) {
		return
	}
	pass.Call("drawIndirect", e.drawIndirect, 0)
	if hasClip {
		pass.Call("setScissorRect", 0, 0, targetW, targetH)
	}
}

// endFrame reclaims emitters that have not appeared in the command stream
// for particlegpu.TTL frames.
func (g *gpuParticles) endFrame() {
	for key, e := range g.emitters {
		if g.frame-e.lastSeen >= particlegpu.TTL {
			e.destroy()
			delete(g.emitters, key)
		}
	}
	g.frame++
}

func (g *gpuParticles) bufferBytes() uint64 {
	const perEmitterOverhead = particlegpu.UniformSize + particlegpu.DrawArgsSize + 16 // counters
	var total uint64
	for _, emitter := range g.emitters {
		// Hot/cold particle state plus survivor/dead index lists. This
		// excludes shared pipelines, textures and browser/driver allocations.
		total += uint64(emitter.capacity*(particlegpu.HotStride+particlegpu.ColdStride+2*4) + perEmitterOverhead)
	}
	return total
}

// destroy releases the storage owned by one emitter. Bind groups are GC'd by
// JavaScript after their buffers become unreachable.
func (e *gpuEmitter) destroy() {
	buffers := []js.Value{
		e.hot,
		e.cold,
		e.alive,
		e.dead,
		e.counters,
		e.drawIndirect,
		e.uniforms,
	}
	for _, buffer := range buffers {
		if buffer.Truthy() {
			buffer.Call("destroy")
		}
	}
}

// applyClipScissor applies the backend-independent particle scissor calculation.
func applyClipScissor(pass js.Value, clip math2d.Rect, camera render.Camera, scale float64, w, h int) bool {
	x, y, width, height, ok := particlegpu.ScissorRect(clip, camera, scale, w, h)
	pass.Call("setScissorRect", x, y, width, height)
	return ok
}

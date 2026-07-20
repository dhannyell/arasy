//go:build js && wasm

package wasm

import (
	"encoding/binary"
	"math"
	"syscall/js"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
	"github.com/dhannyell/frameforge/platform/wasm/gpu"
	"github.com/dhannyell/frameforge/render"
)

const (
	gpuParticleTTL = 300
	// Hot/cold strides must match the WGSL ParticleHot/ParticleCold structs.
	// The split keeps the simulate pass on the 24 mutable bytes; spawn-time
	// state lives in the cold buffer only emit writes and render reads.
	gpuParticleHotStride  = 24
	gpuParticleColdStride = 32
	gpuUniformSize        = 224 // must match the WGSL EmitterUniforms struct
	particleWorkgroup     = 64
	drawBufferSize        = 16 // DrawIndirectArgs: {vertices,instances,firstV,firstI}
	// countersAliveOffset locates the alive cursor inside the counters buffer
	// ({dead, alive, pad, pad}); the host zeroes it before each simulated
	// frame. drawInstanceOffset does the same for the indirect instance count.
	countersAliveOffset = 4
	drawInstanceOffset  = 4
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
	scratch     [gpuUniformSize]byte
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

	emitters map[emitterGPUKey]*gpuEmitter
	frame    uint64

	// timing instrumentation: armTiming runs once per frame before the
	// encodeSim loop, and the first compute pass created afterwards carries
	// the frame-begin timestamp. timingWrote tells the renderer whether that
	// pass actually existed (a fully paused frame encodes none).
	timing      *gpuTiming
	timingArmed bool
	timingWrote bool
}

type emitterGPUKey struct {
	effect  render.ParticleEffectID
	emitter render.EmitterID
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
		emitters: make(map[emitterGPUKey]*gpuEmitter),
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
	g.buildPipelines(r.format, withOffscreen)
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

func (g *gpuParticles) buildPipelines(format js.Value, withOffscreen bool) {
	computeModule := g.device.Call("createShaderModule", map[string]any{"code": particlesComputeWGSL})
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

	renderModule := g.device.Call("createShaderModule", map[string]any{"code": particlesRenderWGSL})
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
	g.drawAlpha = pipeline("fs_main", gpu.PremultipliedBlend(), msaaSampleCount)
	g.drawAdditive = pipeline("fs_main", additiveBlend(), msaaSampleCount)
	g.drawMissing = pipeline("fs_missing", gpu.PremultipliedBlend(), msaaSampleCount)
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

	e.hot = g.buffer(decl.Capacity*gpuParticleHotStride, storage)
	e.cold = g.buffer(decl.Capacity*gpuParticleColdStride, storage)
	e.alive = g.buffer(decl.Capacity*4, storage)
	e.dead = g.buffer(decl.Capacity*4, storage)
	e.counters = g.buffer(16, storage)
	e.drawIndirect = g.buffer(drawBufferSize, storage|usage.Get("INDIRECT").Int())
	e.uniforms = g.buffer(gpuUniformSize, usage.Get("UNIFORM").Int()|usage.Get("COPY_DST").Int())

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
			view = gt.texture.Call("createView")
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
	cmds []render.DrawParticles,
	camera render.Camera,
	w, h int,
) {
	sims := make([]simRequest, 0, len(cmds))
	for _, cmd := range cmds {
		if e, emitCount, ok := g.prepareEmitter(cmd, camera, w, h); ok {
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
		preparePass.Call("dispatchWorkgroups", (sim.emitCount+particleWorkgroup-1)/particleWorkgroup)
	}
	preparePass.Call("end")

	simulatePass := beginComputePass(encoder, sampled, g.timing.simulateTimestampWrites)
	simulatePass.Call("setPipeline", g.simulatePipeline)
	for _, sim := range sims {
		simulatePass.Call("setBindGroup", 0, sim.e.computeBind)
		simulatePass.Call("dispatchWorkgroups", (sim.e.capacity+particleWorkgroup-1)/particleWorkgroup)
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

	key := emitterGPUKey{effect: cmd.Effect, emitter: cmd.Emitter}
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
	packUniforms(e.scratch[:], e, cmd, frame, emitCount, camera, w, h)
	g.queue.Call("writeBuffer", e.uniforms, 0, bytesToJS(e.scratch[:]))

	if cmd.Dt <= 0 {
		return nil, 0, false
	}

	// Queue writes land before any pass in this frame's submission: reset the
	// survivor-list cursor and the indirect instance count so the simulate
	// scan rebuilds both. Skipped on paused frames, which must keep them.
	g.queue.Call("writeBuffer", e.counters, countersAliveOffset, bytesToJS(packU32(0)))
	g.queue.Call("writeBuffer", e.drawIndirect, drawInstanceOffset, bytesToJS(packU32(0)))

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
	e := g.emitters[emitterGPUKey{effect: cmd.Effect, emitter: cmd.Emitter}]
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
// for gpuParticleTTL frames.
func (g *gpuParticles) endFrame() {
	for key, e := range g.emitters {
		if g.frame-e.lastSeen >= gpuParticleTTL {
			e.destroy()
			delete(g.emitters, key)
		}
	}
	g.frame++
}

func (g *gpuParticles) bufferBytes() uint64 {
	const perEmitterOverhead = gpuUniformSize + drawBufferSize + 16 // counters
	var total uint64
	for _, emitter := range g.emitters {
		// Hot/cold particle state plus survivor/dead index lists. This
		// excludes shared pipelines, textures and browser/driver allocations.
		total += uint64(emitter.capacity*(gpuParticleHotStride+gpuParticleColdStride+2*4) + perEmitterOverhead)
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

// packUniforms serializes EmitterUniforms in the exact declaration order of
// particle_compute.wgsl and particle_render.wgsl. The fields are deliberately
// ordered so WGSL uniform alignment introduces no internal padding.
func packUniforms(
	dst []byte,
	e *gpuEmitter,
	cmd render.DrawParticles,
	frame particle.EmissionFrame,
	emitCount int,
	camera render.Camera,
	w, h int,
) {
	d := &e.decl
	scale, offset, screenScale := worldToClip(camera, w, h)
	wobbleAxis := math2d.Vec2{}
	if d.WobbleAmp != 0 {
		angle := d.Direction + cmd.Dir
		wobbleAxis = math2d.Vec2{X: -math.Sin(angle), Y: math.Cos(angle)}
	}

	writer := particleUniformWriter{dst: dst}
	writer.vec4(d.ColorStart)
	writer.vec4(d.ColorEnd)
	writer.vec2(scale)
	writer.vec2(offset)
	writer.vec2(screenScale)
	writer.vec2(math2d.Vec2{X: 2 / float64(w), Y: -2 / float64(h)})
	writer.vec2(cmd.Pos)
	writer.vec2(frame.PrevPos)
	writer.vec2(d.Gravity)
	writer.vec2(d.EmitExtent)
	writer.vec2(particleEmitScale(cmd.EmitScale))
	writer.vec2(wobbleAxis)
	writer.f32(cmd.Dt)
	writer.f32(d.Drag)
	writer.u32(uint32(emitCount))
	writer.u32(frame.Base)
	writer.u32(uint32(e.seed))
	writer.u32(uint32(e.seed >> 32))
	writer.f32(d.Direction + cmd.Dir)
	writer.f32(d.Spread)
	writer.f32(d.Speed.Min)
	writer.f32(d.Speed.Max)
	writer.f32(d.Lifetime.Min)
	writer.f32(d.Lifetime.Max)
	writer.f32(d.SizeStart.Min)
	writer.f32(d.SizeStart.Max)
	writer.f32(d.SizeEnd.Min)
	writer.f32(d.SizeEnd.Max)
	writer.f32(d.Spin)
	writer.f32(d.Rotation.Min)
	writer.f32(d.Rotation.Max)
	writer.f32(d.WobbleAmp)
	writer.f32(d.WobbleFreq)
	writer.f32(d.FadeIn)
	writer.f32(d.FlipXFreq)
	writer.u32(uint32(d.Shape))
	writer.u32(uint32(d.VelMode))
	writer.f32(d.EmitRadius)
	writer.u32(uint32(e.capacity))
}

func packSpriteUniforms(dst []byte, camera render.Camera, w, h int) {
	scale, offset, screenScale := worldToClip(camera, w, h)
	writer := particleUniformWriter{dst: dst}
	writer.vec2(scale)
	writer.vec2(offset)
	writer.vec2(screenScale)
	writer.vec2(math2d.Vec2{X: 2 / float64(w), Y: -2 / float64(h)})
}

type particleUniformWriter struct {
	dst    []byte
	offset int
}

func (w *particleUniformWriter) f32(value float64) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], math.Float32bits(float32(value)))
	w.offset += 4
}

func (w *particleUniformWriter) u32(value uint32) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], value)
	w.offset += 4
}

func (w *particleUniformWriter) vec2(value math2d.Vec2) {
	w.f32(value.X)
	w.f32(value.Y)
}

func (w *particleUniformWriter) vec4(value [4]float64) {
	for _, component := range value {
		w.f32(component)
	}
}

// worldToClip returns the affine coefficients consumed by the particle
// vertex shader. It mirrors Camera.WorldToScreen followed by tess's
// screenToClip conversion, including aspect-fit/expand view resolution.
func worldToClip(camera render.Camera, w, h int) (scale, offset, screenScale math2d.Vec2) {
	if w <= 0 || h <= 0 {
		return math2d.Vec2{}, math2d.Vec2{}, math2d.Vec2{}
	}
	resolved := camera.Resolve()
	screenScale = math2d.Vec2{X: resolved.ScaleX, Y: resolved.ScaleY}
	scale = math2d.Vec2{
		X: 2 * resolved.ScaleX / float64(w),
		Y: -2 * resolved.ScaleY / float64(h),
	}
	offset = math2d.Vec2{
		X: 2*(resolved.Viewport.Min.X-resolved.World.Min.X*resolved.ScaleX)/float64(w) - 1,
		Y: 1 - 2*(resolved.Viewport.Min.Y-resolved.World.Min.Y*resolved.ScaleY)/float64(h),
	}
	return scale, offset, screenScale
}

// applyClipScissor maps a world-space clip to target pixels: camera screen
// coordinates times scale, clamped to the target dimensions (scale is 1 for
// the main pass, Config.ParticleOffscreenScale for offscreen targets). A zero
// clip restores the full target. It returns false for an empty clipped region
// so the caller can skip the indirect draw.
func applyClipScissor(pass js.Value, clip math2d.Rect, camera render.Camera, scale float64, w, h int) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	if clip.Width() <= 0 || clip.Height() <= 0 {
		pass.Call("setScissorRect", 0, 0, w, h)
		return true
	}

	minPoint := camera.WorldToScreen(clip.Min)
	maxPoint := camera.WorldToScreen(clip.Max)
	x0 := max(0, min(w, int(math.Floor(math.Min(minPoint.X, maxPoint.X)*scale))))
	y0 := max(0, min(h, int(math.Floor(math.Min(minPoint.Y, maxPoint.Y)*scale))))
	x1 := max(0, min(w, int(math.Ceil(math.Max(minPoint.X, maxPoint.X)*scale))))
	y1 := max(0, min(h, int(math.Ceil(math.Max(minPoint.Y, maxPoint.Y)*scale))))
	if x1 <= x0 || y1 <= y0 {
		pass.Call("setScissorRect", 0, 0, w, h)
		return false
	}
	pass.Call("setScissorRect", x0, y0, x1-x0, y1-y0)
	return true
}

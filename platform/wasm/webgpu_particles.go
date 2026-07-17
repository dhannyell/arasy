//go:build js && wasm

package wasm

import (
	"encoding/binary"
	"math"
	"syscall/js"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
	"github.com/dhannyell/frameforge/render"
)

const (
	gpuParticleTTL     = 300
	gpuParticleStride  = 48  // must match the WGSL Particle struct
	gpuUniformSize     = 208 // must match the WGSL EmitterUniforms struct
	particleWorkgroup  = 64
	dispatchBufferSize = 12 // DispatchWorkgroupsIndirectArgs: {x,y,z}
	drawBufferSize     = 16 // DrawIndirectArgs: {vertices,instances,firstV,firstI}
)

type gpuEmitter struct {
	decl     particle.EffectDecl
	seed     uint64
	emission particle.EmissionState
	capacity int
	lastSeen uint64
	// missingTexture selects the untinted magenta placeholder pipeline.
	missingTexture bool
	// simulated marks whether this frame ran the compute passes; a paused
	// frame (dt <= 0) skips them and must also skip the list swap so the
	// draw keeps showing the last simulated state (freeze, like the CPU).
	simulated bool
	front     int // which alive list is "current" this frame (0 or 1)

	particles        js.Value // capacity × 48 B
	alive            [2]js.Value
	dead             js.Value
	counters         js.Value // {dead, aliveCur, aliveNew, pad} atomics
	dispatchIndirect js.Value // STORAGE in kick; INDIRECT in simulate pass
	drawIndirect     js.Value // STORAGE in compute; INDIRECT in render pass
	uniforms         js.Value

	prepareBind  [2]js.Value // kick + emit; dispatch args are writable storage
	simulateBind [2]js.Value // simulate; dispatch args are deliberately absent
	renderBind   [2]js.Value // reads alive[1-front] (the survivors list)
	textureBind  js.Value
	scratch      [gpuUniformSize]byte
}

type gpuParticles struct {
	device, queue js.Value
	renderer      *webgpuRenderer

	kickPipeline     js.Value
	emitPipeline     js.Value
	simulatePipeline js.Value
	drawAlpha        js.Value // premultiplied source-over, MSAA 4x
	drawAdditive     js.Value // srcFactor one / dstFactor one, MSAA 4x
	drawMissing      js.Value // opaque magenta, deliberately ignores tint

	prepareLayout  js.Value
	simulateLayout js.Value
	renderLayout   js.Value
	textureLayout  js.Value

	whiteTexture   js.Value // 1×1 bind target for untextured effects
	missingTexture js.Value // magenta 1×1 for a declared texture that failed
	sampler        js.Value

	emitters map[emitterGPUKey]*gpuEmitter
	frame    uint64
}

type emitterGPUKey struct {
	effect  render.ParticleEffectID
	emitter render.EmitterID
}

// newGpuParticles builds the shared particle GPU state once: bind group
// layouts, the three compute pipelines, both render blend variants, the
// white 1×1 texture for untextured effects and the sampler.
func newGpuParticles(r *webgpuRenderer) *gpuParticles {
	g := &gpuParticles{
		device:   r.device,
		queue:    r.queue,
		renderer: r,
		emitters: make(map[emitterGPUKey]*gpuEmitter),
	}
	g.prepareLayout = g.prepareBindLayout()
	g.simulateLayout = g.simulateBindLayout()
	g.renderLayout = g.renderBindLayout()
	g.textureLayout = g.textureBindLayout()
	g.whiteTexture = g.createSolidTexture([4]byte{255, 255, 255, 255})
	g.missingTexture = g.createSolidTexture([4]byte{255, 0, 255, 255})
	g.sampler = g.device.Call("createSampler", map[string]any{
		"magFilter": "linear",
		"minFilter": "linear",
	})
	g.buildPipelines(r.format)
	return g
}

// prepareBindLayout is shared by kick and emit. Both run in the preparation
// pass, where the dispatch-indirect buffer is exclusively writable storage.
func (g *gpuParticles) prepareBindLayout() js.Value {
	compute := js.Global().Get("GPUShaderStage").Get("COMPUTE").Int()
	storage := map[string]any{"type": "storage"}
	entry := func(binding int, buffer map[string]any) map[string]any {
		return map[string]any{"binding": binding, "visibility": compute, "buffer": buffer}
	}
	return g.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			entry(0, storage), // particle pool
			entry(1, storage), // alive current
			entry(2, storage), // alive next (survivors)
			entry(3, storage), // dead list
			entry(4, storage), // counters
			entry(5, storage), // draw indirect args
			entry(6, storage), // dispatch indirect args
			entry(7, map[string]any{"type": "uniform"}),
		},
	})
}

// simulateBindLayout deliberately omits binding 6. The dispatch-indirect
// buffer is consumed by dispatchWorkgroupsIndirect in this pass, so binding
// it as writable storage too would violate WebGPU's usage-scope rules even
// though the simulate entry point never accesses that WGSL global.
func (g *gpuParticles) simulateBindLayout() js.Value {
	compute := js.Global().Get("GPUShaderStage").Get("COMPUTE").Int()
	storage := map[string]any{"type": "storage"}
	entry := func(binding int, buffer map[string]any) map[string]any {
		return map[string]any{"binding": binding, "visibility": compute, "buffer": buffer}
	}
	return g.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			entry(0, storage), // particle pool
			entry(1, storage), // alive current
			entry(2, storage), // alive next (survivors)
			entry(3, storage), // dead list
			entry(4, storage), // counters
			entry(5, storage), // draw indirect args
			entry(7, map[string]any{"type": "uniform"}),
		},
	})
}

// renderBindLayout feeds the vertex-pulling shader: uniforms, pool and the
// survivors list, all read-only (vertex stages cannot write storage).
func (g *gpuParticles) renderBindLayout() js.Value {
	vertex := js.Global().Get("GPUShaderStage").Get("VERTEX").Int()
	readOnly := map[string]any{"type": "read-only-storage"}
	return g.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			map[string]any{"binding": 0, "visibility": vertex, "buffer": map[string]any{"type": "uniform"}},
			map[string]any{"binding": 1, "visibility": vertex, "buffer": readOnly},
			map[string]any{"binding": 2, "visibility": vertex, "buffer": readOnly},
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

func (g *gpuParticles) buildPipelines(format js.Value) {
	computeModule := g.device.Call("createShaderModule", map[string]any{"code": particlesComputeWGSL})
	preparePipelineLayout := g.device.Call("createPipelineLayout", map[string]any{
		"bindGroupLayouts": []any{g.prepareLayout},
	})
	preparePipeline := func(entry string) js.Value {
		return g.device.Call("createComputePipeline", map[string]any{
			"layout":  preparePipelineLayout,
			"compute": map[string]any{"module": computeModule, "entryPoint": entry},
		})
	}
	g.kickPipeline = preparePipeline("kick")
	g.emitPipeline = preparePipeline("emit")
	simulatePipelineLayout := g.device.Call("createPipelineLayout", map[string]any{
		"bindGroupLayouts": []any{g.simulateLayout},
	})
	g.simulatePipeline = g.device.Call("createComputePipeline", map[string]any{
		"layout":  simulatePipelineLayout,
		"compute": map[string]any{"module": computeModule, "entryPoint": "simulate"},
	})

	renderModule := g.device.Call("createShaderModule", map[string]any{"code": particlesRenderWGSL})
	renderLayout := g.device.Call("createPipelineLayout", map[string]any{
		"bindGroupLayouts": []any{g.renderLayout, g.textureLayout},
	})
	pipeline := func(fragment string, blend map[string]any) js.Value {
		return g.device.Call("createRenderPipeline", map[string]any{
			"layout": renderLayout,
			"vertex": map[string]any{"module": renderModule, "entryPoint": "vs_main"},
			"fragment": map[string]any{
				"module":     renderModule,
				"entryPoint": fragment,
				"targets":    []any{map[string]any{"format": format, "blend": blend}},
			},
			"primitive":   map[string]any{"topology": "triangle-list"},
			"multisample": map[string]any{"count": msaaSampleCount},
		})
	}
	g.drawAlpha = pipeline("fs_main", premultipliedBlend())
	g.drawAdditive = pipeline("fs_main", additiveBlend())
	g.drawMissing = pipeline("fs_missing", premultipliedBlend())
}

func (g *gpuParticles) createSolidTexture(color [4]byte) js.Value {
	texture := g.device.Call("createTexture", map[string]any{
		"size":   []any{1, 1},
		"format": "rgba8unorm",
		"usage":  textureUsageTextureBinding() | textureUsageCopyDst(),
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

func (g *gpuParticles) newEmitter(decl particle.EffectDecl, seed uint64) *gpuEmitter {
	e := &gpuEmitter{decl: decl, seed: seed, capacity: decl.Capacity}
	usage := js.Global().Get("GPUBufferUsage")
	storage := usage.Get("STORAGE").Int() | usage.Get("COPY_DST").Int()

	e.particles = g.buffer(decl.Capacity*gpuParticleStride, storage)
	e.alive[0] = g.buffer(decl.Capacity*4, storage)
	e.alive[1] = g.buffer(decl.Capacity*4, storage)
	e.dead = g.buffer(decl.Capacity*4, storage)
	e.counters = g.buffer(16, storage)
	e.dispatchIndirect = g.buffer(dispatchBufferSize, storage|usage.Get("INDIRECT").Int())
	e.drawIndirect = g.buffer(drawBufferSize, storage|usage.Get("INDIRECT").Int())
	e.uniforms = g.buffer(gpuUniformSize, usage.Get("UNIFORM").Int()|usage.Get("COPY_DST").Int())

	// Dead list starts full (every slot free), counters at {capacity,0,0},
	// dispatch at {0,1,1}, and draw at {6,0,0,0}: dispatch Y/Z fixed at 1,
	// six vertices per instance, zero instances until simulate runs.
	init := make([]byte, decl.Capacity*4)
	for i := 0; i < decl.Capacity; i++ {
		binary.LittleEndian.PutUint32(init[i*4:], uint32(i))
	}
	g.queue.Call("writeBuffer", e.dead, 0, bytesToJS(init))
	g.queue.Call("writeBuffer", e.counters, 0, bytesToJS(packU32(uint32(decl.Capacity), 0, 0, 0)))
	g.queue.Call("writeBuffer", e.dispatchIndirect, 0, bytesToJS(packU32(0, 1, 1)))
	g.queue.Call("writeBuffer", e.drawIndirect, 0, bytesToJS(packU32(6, 0, 0, 0)))

	// Both ping-pong orientations get their bind groups once; the per-frame
	// swap is an index flip, never a bind-group rebuild.
	for o := 0; o < 2; o++ {
		e.prepareBind[o] = g.bindPrepare(e, e.alive[o], e.alive[1-o])
		e.simulateBind[o] = g.bindSimulate(e, e.alive[o], e.alive[1-o])
		e.renderBind[o] = g.bindRender(e, e.alive[1-o])
	}
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

// bindPrepare builds the kick/emit bind group for one ping-pong orientation:
// cur is the alive list the emit pass pushes into and simulate reads; next
// is the survivors list simulate writes. Binding numbers mirror the
// @group(0) declarations in the particle WGSL.
func (g *gpuParticles) bindPrepare(e *gpuEmitter, cur, next js.Value) js.Value {
	return g.device.Call("createBindGroup", map[string]any{
		"layout": g.prepareLayout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": map[string]any{"buffer": e.particles}},
			map[string]any{"binding": 1, "resource": map[string]any{"buffer": cur}},
			map[string]any{"binding": 2, "resource": map[string]any{"buffer": next}},
			map[string]any{"binding": 3, "resource": map[string]any{"buffer": e.dead}},
			map[string]any{"binding": 4, "resource": map[string]any{"buffer": e.counters}},
			map[string]any{"binding": 5, "resource": map[string]any{"buffer": e.drawIndirect}},
			map[string]any{"binding": 6, "resource": map[string]any{"buffer": e.dispatchIndirect}},
			map[string]any{"binding": 7, "resource": map[string]any{"buffer": e.uniforms}},
		},
	})
}

// bindSimulate mirrors bindPrepare without binding 6. That buffer belongs
// exclusively to dispatchWorkgroupsIndirect during the simulation pass.
func (g *gpuParticles) bindSimulate(e *gpuEmitter, cur, next js.Value) js.Value {
	return g.device.Call("createBindGroup", map[string]any{
		"layout": g.simulateLayout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": map[string]any{"buffer": e.particles}},
			map[string]any{"binding": 1, "resource": map[string]any{"buffer": cur}},
			map[string]any{"binding": 2, "resource": map[string]any{"buffer": next}},
			map[string]any{"binding": 3, "resource": map[string]any{"buffer": e.dead}},
			map[string]any{"binding": 4, "resource": map[string]any{"buffer": e.counters}},
			map[string]any{"binding": 5, "resource": map[string]any{"buffer": e.drawIndirect}},
			map[string]any{"binding": 7, "resource": map[string]any{"buffer": e.uniforms}},
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

// bindRender builds the vertex-pulling bind group for one ping-pong
// orientation: survivors is the alive list the simulate pass just wrote,
// which the vertex shader indexes by instance_index. Storage buffers are
// read-only here: core WebGPU forbids writable storage in vertex stages.
func (g *gpuParticles) bindRender(e *gpuEmitter, survivors js.Value) js.Value {
	return g.device.Call("createBindGroup", map[string]any{
		"layout": g.renderLayout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": map[string]any{"buffer": e.uniforms}},
			map[string]any{"binding": 1, "resource": map[string]any{"buffer": e.particles}},
			map[string]any{"binding": 2, "resource": map[string]any{"buffer": survivors}},
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

// encodeSim updates one emitter's CPU accounting and records its kick, emit
// and simulate dispatches before the frame's render pass. The same emitter
// may be drawn more than once in painter order, but it is simulated at most
// once per frame.
func (g *gpuParticles) encodeSim(
	encoder js.Value,
	cmd render.DrawParticles,
	camera render.Camera,
	w, h int,
) {
	decl, ok := g.renderer.manifest.EffectDecl(cmd.Effect)
	if !ok {
		return
	}

	key := emitterGPUKey{effect: cmd.Effect, emitter: cmd.Emitter}
	e, exists := g.emitters[key]
	// The first command for one emitter key owns simulation this frame. In
	// particular, never destroy/recreate buffers for a later command with a
	// different seed after the encoder has already referenced the old ones.
	if exists && e.lastSeen == g.frame {
		return
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

	// Pause/scrub updates LastPos and the draw transform but freezes particle
	// buffers and the ping-pong orientation, matching particle.Emitter.Step.
	if cmd.Dt <= 0 {
		return
	}

	// The GPU consumes the full requested hash range even when its dead stack
	// is exhausted and some emit invocations drop their spawn.
	e.emission.Emitted += uint32(emitCount)
	e.simulated = true

	preparePass := encoder.Call("beginComputePass")
	preparePass.Call("setBindGroup", 0, e.prepareBind[e.front])

	preparePass.Call("setPipeline", g.kickPipeline)
	preparePass.Call("dispatchWorkgroups", 1)

	if emitCount > 0 {
		preparePass.Call("setPipeline", g.emitPipeline)
		preparePass.Call("dispatchWorkgroups", (emitCount+particleWorkgroup-1)/particleWorkgroup)
	}
	preparePass.Call("end")

	// WebGPU forbids one buffer from being writable STORAGE and INDIRECT in
	// the same usage scope. A new pass makes kick's dispatch-buffer writes
	// visible, while simulate writes only the separate draw-args buffer.
	simulatePass := encoder.Call("beginComputePass")
	simulatePass.Call("setBindGroup", 0, e.simulateBind[e.front])
	simulatePass.Call("setPipeline", g.simulatePipeline)
	simulatePass.Call("dispatchWorkgroupsIndirect", e.dispatchIndirect, 0)
	simulatePass.Call("end")
}

// draw submits one emitter at its exact painter-order position. The indirect
// instance count was produced by simulate, so no GPU readback is needed.
func (g *gpuParticles) draw(
	pass js.Value,
	cmd render.DrawParticles,
	camera render.Camera,
	w, h int,
) {
	e := g.emitters[emitterGPUKey{effect: cmd.Effect, emitter: cmd.Emitter}]
	if e == nil || e.seed != cmd.Seed || e.lastSeen != g.frame {
		return
	}

	pipeline := g.drawAlpha
	if e.missingTexture {
		pipeline = g.drawMissing
	} else if e.decl.Blend == particle.BlendAdditive {
		pipeline = g.drawAdditive
	}
	renderOrientation := e.front
	if !e.simulated {
		// After endFrame, front names the last survivors list. renderBind[o]
		// reads alive[1-o], so a frozen frame uses the opposite prebuilt bind.
		renderOrientation = 1 - e.front
	}
	pass.Call("setPipeline", pipeline)
	pass.Call("setBindGroup", 0, e.renderBind[renderOrientation])
	pass.Call("setBindGroup", 1, e.textureBind)

	hasClip := cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0
	if !applyClipScissor(pass, cmd.Clip, camera, w, h) {
		return
	}
	pass.Call("drawIndirect", e.drawIndirect, 0)
	if hasClip {
		pass.Call("setScissorRect", 0, 0, w, h)
	}
}

// endFrame commits ping-pong orientation changes and reclaims emitters that
// have not appeared in the command stream for gpuParticleTTL frames.
func (g *gpuParticles) endFrame() {
	for key, e := range g.emitters {
		if e.simulated {
			e.front = 1 - e.front
			e.simulated = false
		}
		if g.frame-e.lastSeen >= gpuParticleTTL {
			e.destroy()
			delete(g.emitters, key)
		}
	}
	g.frame++
}

// destroy releases the storage owned by one emitter. Bind groups are GC'd by
// JavaScript after their buffers become unreachable.
func (e *gpuEmitter) destroy() {
	buffers := []js.Value{
		e.particles,
		e.alive[0],
		e.alive[1],
		e.dead,
		e.counters,
		e.dispatchIndirect,
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
	writer.u32(uint32(d.Shape))
	writer.u32(uint32(d.VelMode))
	writer.f32(d.EmitRadius)
	writer.u32(uint32(e.capacity))
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

// applyClipScissor maps a world-space clip to WebGPU framebuffer pixels. A
// zero clip restores the full target. It returns false for an empty clipped
// region so the caller can skip the indirect draw.
func applyClipScissor(pass js.Value, clip math2d.Rect, camera render.Camera, w, h int) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	if clip.Width() <= 0 || clip.Height() <= 0 {
		pass.Call("setScissorRect", 0, 0, w, h)
		return true
	}

	minPoint := camera.WorldToScreen(clip.Min)
	maxPoint := camera.WorldToScreen(clip.Max)
	x0 := max(0, min(w, int(math.Floor(math.Min(minPoint.X, maxPoint.X)))))
	y0 := max(0, min(h, int(math.Floor(math.Min(minPoint.Y, maxPoint.Y)))))
	x1 := max(0, min(w, int(math.Ceil(math.Max(minPoint.X, maxPoint.X)))))
	y1 := max(0, min(h, int(math.Ceil(math.Max(minPoint.Y, maxPoint.Y)))))
	if x1 <= x0 || y1 <= y0 {
		pass.Call("setScissorRect", 0, 0, w, h)
		return false
	}
	pass.Call("setScissorRect", x0, y0, x1-x0, y1-y0)
	return true
}

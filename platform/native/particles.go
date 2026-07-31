//go:build windows && amd64

package native

import (
	"encoding/binary"
	"fmt"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/render/gpu/particlegpu"
	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
	"github.com/oliverbestmann/webgpu/wgpu"
)

type gpuEmitter struct {
	decl                                                     particle.EffectDecl
	seed                                                     uint64
	emission                                                 particle.EmissionState
	capacity                                                 int
	lastSeen                                                 uint64
	missingTexture                                           bool
	bufferBytes                                              uint64
	hot, cold, alive, dead, counters, drawIndirect, uniforms *wgpu.Buffer
	computeBind, renderBind, textureBind                     *wgpu.BindGroup
	scratch                                                  [particlegpu.UniformSize]byte
}
type particleSim struct {
	e    *gpuEmitter
	emit int
}
type gpuParticles struct {
	device                                     *wgpu.Device
	queue                                      *wgpu.Queue
	manifest                                   asset.Manifest
	textures                                   []gpuTexture
	emitPipeline, simulatePipeline             *wgpu.ComputePipeline
	drawAlpha, drawAdditive, drawMissing       *wgpu.RenderPipeline
	computeLayout, renderLayout, textureLayout *wgpu.BindGroupLayout
	white, magenta                             *wgpu.Texture
	whiteView, magentaView                     *wgpu.TextureView
	sampler                                    *wgpu.Sampler
	emitters                                   map[particlegpu.EmitterKey]*gpuEmitter
	frame                                      uint64
}

func newGpuParticles(
	device *wgpu.Device,
	queue *wgpu.Queue,
	manifest asset.Manifest,
	textures []gpuTexture,
	format wgpu.TextureFormat,
	sampleCount uint32,
) (*gpuParticles, error) {
	g := &gpuParticles{
		device:   device,
		queue:    queue,
		manifest: manifest,
		textures: textures,
		emitters: make(map[particlegpu.EmitterKey]*gpuEmitter),
	}
	var err error
	storage := func(binding uint32, visibility wgpu.ShaderStage, typ wgpu.BufferBindingType) wgpu.BindGroupLayoutEntry {
		return wgpu.BindGroupLayoutEntry{
			Binding:    binding,
			Visibility: visibility,
			Buffer:     wgpu.BufferBindingLayout{Type: typ},
		}
	}
	if g.computeLayout, err = device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			storage(0, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeStorage),
			storage(1, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeStorage),
			storage(2, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeStorage),
			storage(3, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeStorage),
			storage(4, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeStorage),
			storage(5, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeUniform),
			storage(6, wgpu.ShaderStageCompute, wgpu.BufferBindingTypeStorage),
		},
	}); err != nil {
		return nil, fmt.Errorf("create particle compute layout: %w", err)
	}
	if g.renderLayout, err = device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			storage(0, wgpu.ShaderStageVertex, wgpu.BufferBindingTypeUniform),
			storage(1, wgpu.ShaderStageVertex, wgpu.BufferBindingTypeReadOnlyStorage),
			storage(2, wgpu.ShaderStageVertex, wgpu.BufferBindingTypeReadOnlyStorage),
			storage(3, wgpu.ShaderStageVertex, wgpu.BufferBindingTypeReadOnlyStorage),
		},
	}); err != nil {
		g.close()
		return nil, err
	}
	if g.textureLayout, err = device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2D,
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageFragment,
				Sampler:    wgpu.SamplerBindingLayout{Type: wgpu.SamplerBindingTypeFiltering},
			},
		},
	},
	); err != nil {
		g.close()
		return nil, err
	}
	g.white, g.whiteView, err = g.solidTexture([4]byte{255, 255, 255, 255})
	if err != nil {
		g.close()
		return nil, err
	}
	g.magenta, g.magentaView, err = g.solidTexture([4]byte{255, 0, 255, 255})
	if err != nil {
		g.close()
		return nil, err
	}
	g.sampler, err = device.TryCreateSampler(
		&wgpu.SamplerDescriptor{MagFilter: wgpu.FilterModeLinear, MinFilter: wgpu.FilterModeLinear, MaxAnisotropy: 1},
	)
	if err != nil {
		g.close()
		return nil, err
	}
	computeLayout, err := device.TryCreatePipelineLayout(
		&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{g.computeLayout}},
	)
	if err != nil {
		g.close()
		return nil, err
	}
	defer computeLayout.Release()
	cm, err := device.TryCreateShaderModule(
		&wgpu.ShaderModuleDescriptor{WGSLSource: &wgpu.ShaderSourceWGSL{Code: shaders.ParticlesComputeWGSL}},
	)
	if err != nil {
		g.close()
		return nil, err
	}
	defer cm.Release()
	g.emitPipeline, err = device.TryCreateComputePipeline(
		&wgpu.ComputePipelineDescriptor{
			Layout:  computeLayout,
			Compute: wgpu.ProgrammableStageDescriptor{Module: cm, EntryPoint: "emit"},
		})
	if err != nil {
		g.close()
		return nil, err
	}
	g.simulatePipeline, err = device.TryCreateComputePipeline(
		&wgpu.ComputePipelineDescriptor{
			Layout:  computeLayout,
			Compute: wgpu.ProgrammableStageDescriptor{Module: cm, EntryPoint: "simulate"},
		})
	if err != nil {
		g.close()
		return nil, err
	}
	renderLayout, err := device.TryCreatePipelineLayout(
		&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{g.renderLayout, g.textureLayout}},
	)
	if err != nil {
		g.close()
		return nil, err
	}
	defer renderLayout.Release()
	rm, err := device.TryCreateShaderModule(
		&wgpu.ShaderModuleDescriptor{WGSLSource: &wgpu.ShaderSourceWGSL{Code: shaders.ParticlesRenderWGSL}},
	)
	if err != nil {
		g.close()
		return nil, err
	}
	defer rm.Release()
	makePipe := func(entry string, blend *wgpu.BlendState) (*wgpu.RenderPipeline, error) {
		return device.TryCreateRenderPipeline(
			&wgpu.RenderPipelineDescriptor{
				Layout:      renderLayout,
				Vertex:      wgpu.VertexState{Module: rm, EntryPoint: "vs_main"},
				Primitive:   wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList},
				Multisample: wgpu.MultisampleState{Count: sampleCount, Mask: ^uint32(0)},
				Fragment: &wgpu.FragmentState{
					Module:     rm,
					EntryPoint: entry,
					Targets:    []wgpu.ColorTargetState{{Format: format, Blend: blend, WriteMask: wgpu.ColorWriteMaskAll}},
				},
			},
		)
	}
	if g.drawAlpha, err = makePipe("fs_main", premultipliedBlend()); err != nil {
		g.close()
		return nil, err
	}
	if g.drawAdditive, err = makePipe("fs_main", additiveBlend()); err != nil {
		g.close()
		return nil, err
	}
	if g.drawMissing, err = makePipe("fs_missing", premultipliedBlend()); err != nil {
		g.close()
		return nil, err
	}
	return g, nil
}

func (g *gpuParticles) solidTexture(px [4]byte) (*wgpu.Texture, *wgpu.TextureView, error) {
	t, e := g.device.TryCreateTexture(
		&wgpu.TextureDescriptor{
			Usage: wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst, Dimension: wgpu.TextureDimension2D,
			Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			Format:        wgpu.TextureFormatRGBA8Unorm,
			MipLevelCount: 1,
			SampleCount:   1},
	)
	if e != nil {
		return nil, nil, e
	}
	if e = g.queue.TryWriteTexture(
		&wgpu.TexelCopyTextureInfo{
			Texture: t, Aspect: wgpu.TextureAspectAll,
		},
		px[:],
		&wgpu.TexelCopyBufferLayout{BytesPerRow: 4, RowsPerImage: 1},
		&wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	); e != nil {
		t.Release()
		return nil, nil, e
	}
	v, e := t.TryCreateView(nil)
	return t, v, e
}

func (g *gpuParticles) bufferBytes() uint64 {
	var total uint64
	for _, emitter := range g.emitters {
		total += emitter.bufferBytes
	}
	return total
}

func (g *gpuParticles) newEmitter(d particle.EffectDecl, seed uint64) (*gpuEmitter, error) {
	e := &gpuEmitter{decl: d, seed: seed, capacity: d.Capacity}
	mk := func(size int, usage wgpu.BufferUsage) (*wgpu.Buffer, error) {
		e.bufferBytes += uint64(size)
		return g.device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(size), Usage: usage})
	}
	var err error
	st := wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst
	if e.hot, err = mk(d.Capacity*particlegpu.HotStride, st); err != nil {
		return nil, err
	}
	if e.cold, err = mk(d.Capacity*particlegpu.ColdStride, st); err != nil {
		e.destroy()
		return nil, err
	}
	for _, x := range []**wgpu.Buffer{&e.alive, &e.dead} {
		*x, err = mk(d.Capacity*4, st)
		if err != nil {
			e.destroy()
			return nil, err
		}
	}
	if e.counters, err = mk(16, st); err != nil {
		e.destroy()
		return nil, err
	}
	if e.drawIndirect, err = mk(particlegpu.DrawArgsSize, st|wgpu.BufferUsageIndirect); err != nil {
		e.destroy()
		return nil, err
	}
	if e.uniforms, err = mk(particlegpu.UniformSize, wgpu.BufferUsageUniform|wgpu.BufferUsageCopyDst); err != nil {
		e.destroy()
		return nil, err
	}
	init := make([]byte, d.Capacity*4)
	for i := range d.Capacity {
		binary.LittleEndian.PutUint32(init[i*4:], uint32(i))
	}
	if err = g.queue.TryWriteBuffer(e.dead, 0, init); err != nil {
		e.destroy()
		return nil, err
	}
	var args [16]byte
	binary.LittleEndian.PutUint32(args[:], uint32(d.Capacity))
	if err = g.queue.TryWriteBuffer(e.counters, 0, args[:]); err != nil {
		e.destroy()
		return nil, err
	}
	binary.LittleEndian.PutUint32(args[:], 3)
	binary.LittleEndian.PutUint32(args[4:], 0)
	if err = g.queue.TryWriteBuffer(e.drawIndirect, 0, args[:]); err != nil {
		e.destroy()
		return nil, err
	}
	e.computeBind, err = g.bindCompute(e)
	if err != nil {
		e.destroy()
		return nil, err
	}
	e.renderBind, err = g.bindRender(e)
	if err != nil {
		e.destroy()
		return nil, err
	}
	e.textureBind, e.missingTexture, err = g.bindTexture(d)
	if err != nil {
		e.destroy()
		return nil, err
	}
	return e, nil
}

func (g *gpuParticles) bindCompute(e *gpuEmitter) (*wgpu.BindGroup, error) {
	return g.device.TryCreateBindGroup(
		&wgpu.BindGroupDescriptor{
			Layout: g.computeLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Buffer: e.hot, Size: wgpu.WholeSize},
				{Binding: 1, Buffer: e.alive, Size: wgpu.WholeSize},
				{Binding: 2, Buffer: e.dead, Size: wgpu.WholeSize},
				{Binding: 3, Buffer: e.counters, Size: wgpu.WholeSize},
				{Binding: 4, Buffer: e.drawIndirect, Size: wgpu.WholeSize},
				{Binding: 5, Buffer: e.uniforms, Size: wgpu.WholeSize},
				{Binding: 6, Buffer: e.cold, Size: wgpu.WholeSize}},
		},
	)
}

func (g *gpuParticles) bindRender(e *gpuEmitter) (*wgpu.BindGroup, error) {
	return g.device.TryCreateBindGroup(
		&wgpu.BindGroupDescriptor{
			Layout: g.renderLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Buffer: e.uniforms, Size: wgpu.WholeSize},
				{Binding: 1, Buffer: e.hot, Size: wgpu.WholeSize},
				{Binding: 2, Buffer: e.alive, Size: wgpu.WholeSize},
				{Binding: 3, Buffer: e.cold, Size: wgpu.WholeSize}},
		},
	)
}

func (g *gpuParticles) bindTexture(d particle.EffectDecl) (*wgpu.BindGroup, bool, error) {
	v := g.whiteView
	missing := false
	if d.Texture != "" {
		missing = true
		v = g.magentaView
		if t, ok := textureFor(g.textures, g.manifest.Texture(d.Texture)); ok {
			v = t.view
			missing = false
		}
	}
	b, e := g.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: g.textureLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: v},
			{Binding: 1, Sampler: g.sampler},
		},
	})
	return b, missing, e
}

func (g *gpuParticles) prepare(cmd render.DrawParticles, c render.Camera, w, h int) (particleSim, bool) {
	d, ok := g.manifest.EffectDecl(cmd.Effect)
	if !ok {
		return particleSim{}, false
	}
	key := particlegpu.EmitterKey{Effect: cmd.Effect, Emitter: cmd.Emitter}
	e := g.emitters[key]
	if e != nil && e.lastSeen == g.frame {
		return particleSim{}, false
	}
	if e == nil || e.seed != cmd.Seed {
		if e != nil {
			e.destroy()
		}
		var err error
		e, err = g.newEmitter(d, cmd.Seed)
		if err != nil {
			return particleSim{}, false
		}
		g.emitters[key] = e
	}
	e.lastSeen = g.frame
	f := e.emission.Advance(cmd.Pos, d.EmitRate, cmd.Intensity, d.BurstSize, cmd.Burst, cmd.Dt)
	n := max(f.Count, 0)
	particlegpu.PackUniforms(e.scratch[:], d, cmd, f, n, e.seed, e.capacity, c, w, h)
	if g.queue.TryWriteBuffer(e.uniforms, 0, e.scratch[:]) != nil || cmd.Dt <= 0 {
		return particleSim{}, false
	}
	var zero [4]byte
	_ = g.queue.TryWriteBuffer(e.counters, particlegpu.CountersAliveOffset, zero[:])
	_ = g.queue.TryWriteBuffer(e.drawIndirect, particlegpu.DrawInstanceOffset, zero[:])
	e.emission.Emitted += uint32(n)
	return particleSim{e, n}, true
}

func (g *gpuParticles) encode(encoder *wgpu.CommandEncoder, s []particleSim) error {
	if len(s) == 0 {
		return nil
	}
	p := encoder.BeginComputePass(nil)
	defer p.Release()
	p.SetPipeline(g.emitPipeline)
	for _, v := range s {
		if v.emit > 0 {
			p.SetBindGroup(0, v.e.computeBind, nil)
			p.DispatchWorkgroups(uint32((v.emit+particlegpu.Workgroup-1)/particlegpu.Workgroup), 1, 1)
		}
	}
	if e := p.TryEnd(); e != nil {
		return e
	}
	p = encoder.BeginComputePass(nil)
	defer p.Release()
	p.SetPipeline(g.simulatePipeline)
	for _, v := range s {
		p.SetBindGroup(0, v.e.computeBind, nil)
		p.DispatchWorkgroups(uint32((v.e.capacity+particlegpu.Workgroup-1)/particlegpu.Workgroup), 1, 1)
	}
	return p.TryEnd()
}

func (g *gpuParticles) draw(p *wgpu.RenderPassEncoder, cmd render.DrawParticles, c render.Camera, w, h int) {
	e := g.emitters[particlegpu.EmitterKey{Effect: cmd.Effect, Emitter: cmd.Emitter}]
	if e == nil || e.seed != cmd.Seed || e.lastSeen != g.frame {
		return
	}
	pipe := g.drawAlpha
	if e.missingTexture {
		pipe = g.drawMissing
	} else if e.decl.Blend == particle.BlendAdditive {
		pipe = g.drawAdditive
	}
	p.SetPipeline(pipe)
	p.SetBindGroup(0, e.renderBind, nil)
	p.SetBindGroup(1, e.textureBind, nil)
	x, y, sw, sh, ok := particlegpu.ScissorRect(cmd.Clip, c, 1, w, h)
	p.SetScissorRect(uint32(x), uint32(y), uint32(sw), uint32(sh))
	if ok {
		p.DrawIndirect(e.drawIndirect, 0)
	}
	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		p.SetScissorRect(0, 0, uint32(w), uint32(h))
	}
}

func (g *gpuParticles) endFrame() {
	for k, e := range g.emitters {
		if g.frame-e.lastSeen >= particlegpu.TTL {
			e.destroy()
			delete(g.emitters, k)
		}
	}
	g.frame++
}

func (e *gpuEmitter) destroy() {
	for _, b := range []*wgpu.Buffer{e.hot, e.cold, e.alive, e.dead, e.counters, e.drawIndirect, e.uniforms} {
		releaseBuffer(&b)
	}
	for _, b := range []*wgpu.BindGroup{e.computeBind, e.renderBind, e.textureBind} {
		if b != nil {
			b.Release()
		}
	}
}

func (g *gpuParticles) close() {
	if g == nil {
		return
	}
	for _, e := range g.emitters {
		e.destroy()
	}
	for _, p := range []*wgpu.ComputePipeline{g.emitPipeline, g.simulatePipeline} {
		if p != nil {
			p.Release()
		}
	}
	for _, p := range []*wgpu.RenderPipeline{g.drawAlpha, g.drawAdditive, g.drawMissing} {
		releasePipeline(&p)
	}
	for _, l := range []*wgpu.BindGroupLayout{g.computeLayout, g.renderLayout, g.textureLayout} {
		if l != nil {
			l.Release()
		}
	}
	releaseTextureView(&g.whiteView)
	releaseTextureView(&g.magentaView)
	releaseTexture(&g.white)
	releaseTexture(&g.magenta)
	releaseSampler(&g.sampler)
}

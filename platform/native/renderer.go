//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/diagnostic"
	gpubuffer "github.com/dhannyell/arasy/internal/render/gpu/buffer"
	"github.com/dhannyell/arasy/internal/render/gpu/fontatlas"
	"github.com/dhannyell/arasy/internal/render/gpu/particlegpu"
	"github.com/dhannyell/arasy/internal/render/gpu/shaders"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/go-gl/glfw/v3.4/glfw"
	"github.com/oliverbestmann/webgpu/wgpu"
)

// msaaSampleCount falls back when the mode is invalid or the selected surface
// format rejects an optional sample count.
func msaaSampleCount(
	mode render.MSAAMode,
	formatFeatures bool,
	validate func(uint32) error,
	log *slog.Logger,
) uint32 {
	if log == nil {
		log = slog.Default()
	}
	samples, ok := mode.Samples()
	if !ok {
		log.Warn("unknown MSAA mode, using the default",
			"requested", uint8(mode), "fallback", samples)
	}
	if samples != 1 && samples != 4 {
		var err error
		switch {
		case !formatFeatures:
			err = fmt.Errorf("adapter-specific format features unavailable")
		case validate == nil:
			err = fmt.Errorf("surface format capability probe unavailable")
		default:
			err = validate(uint32(samples))
		}
		if err != nil {
			log.Warn("MSAA level unsupported by this surface format, using the default",
				"requested", samples, "fallback", 4, "err", err)
			samples = 4
		}
	}
	return uint32(samples)
}

// Renderer is the provisional streaming renderer for a native GLFW window.
// A later host API may wrap this constructor with lifecycle and input options.
type Renderer struct {
	device *device
	clear  render.Color
	log    *slog.Logger

	fontProvider  *fontatlas.Provider
	particles     *gpuParticles
	textures      []gpuTexture
	texSizes      tess.TexSizes
	textAtlases   map[tess.AtlasKey]gpuTextAtlas
	spriteBatches map[render.SpriteBatchID]*nativeSpriteBatch
	tess          tess.Builder

	solidPipeline           *wgpu.RenderPipeline
	texturedPipeline        *wgpu.RenderPipeline
	spritePipeline          *wgpu.RenderPipeline
	textGlyphPipeline       *wgpu.RenderPipeline
	spritePipelineLayout    *wgpu.PipelineLayout
	textGlyphPipelineLayout *wgpu.PipelineLayout
	mipPipeline             *wgpu.RenderPipeline
	postSampler             *wgpu.Sampler
	postPipelines           map[string]*wgpu.RenderPipeline
	postUniforms            []*wgpu.Buffer
	postSceneA              nativePostTarget
	postSceneB              nativePostTarget
	postBloom               []nativePostTarget
	postWidth               int
	postHeight              int
	postDisabled            bool
	// postBindGroups caches one bind group per {pipeline, sampled targets,
	// uniform slot}; a steady chain creates none per frame. Released whenever
	// a post target is recreated.
	postBindGroups map[nativePostBindKey]*wgpu.BindGroup
	// postUniformScratch stages one 64-byte uniform write per effect without
	// allocating.
	postUniformScratch [64]byte

	// Additive variants of the command pipelines, selected per draw run when
	// a command sets BlendAdditive. Same shaders, different blend state.
	solidAddPipeline     *wgpu.RenderPipeline
	texturedAddPipeline  *wgpu.RenderPipeline
	spriteAddPipeline    *wgpu.RenderPipeline
	textGlyphAddPipeline *wgpu.RenderPipeline
	textureSampler       *wgpu.Sampler
	mipSampler           *wgpu.Sampler

	solidBuffer           *wgpu.Buffer
	solidCap              int
	texBuffer             *wgpu.Buffer
	texCap                int
	spriteBuffer          *wgpu.Buffer
	spriteCap             int
	textGlyphBuffer       *wgpu.Buffer
	textGlyphCap          int
	textLabelBuffer       *wgpu.Buffer
	textLabelCap          int
	spriteUniform         *wgpu.Buffer
	spriteUniformCap      int
	spriteUniformSlotSize int
	spriteUniformOffsets  []uint32
	spriteUniformEntries  []nativeUniformEntry

	msaaTexture *wgpu.Texture
	msaaView    *wgpu.TextureView
	msaaWidth   int
	msaaHeight  int
	msaaSamples uint32

	// Offscreen particle compositing, active when particleScale > 0: each
	// contiguous run of particle commands renders into one pooled
	// single-sample target and is composited back at its painter-order
	// position by a fullscreen triangle.
	particleScale     float64
	compositeLayout   *wgpu.BindGroupLayout
	compositePipeline *wgpu.RenderPipeline
	compositeSampler  *wgpu.Sampler
	particleTargets   []particleTarget
	particleTargetW   int
	particleTargetH   int

	// timing is nil unless the device granted timestamp-query, and sampling is
	// additionally gated by timeGPU so the option can be toggled at runtime.
	timing  *gpuTiming
	timeGPU bool

	diagnostics diagnostic.Once
	err         error
	// presentedFrame stays false for skipped/reconfigured surfaces and flips
	// only after the first successful surface.Present call.
	presentedFrame     bool
	timeCPU            bool
	cpuTess, cpuUpload time.Duration
	cpuEncode          time.Duration
	cpuPhasesSet       bool
	// cpuFrame separates queue submission, CPU work and surface acquisition.
	cpuFrame cpuFrameTiming
	// presentWait is how long the last frame blocked on presentation
	// (surface acquire plus present). With vsync this is the vblank wait —
	// idle time, not work — so it must be reported apart from the CPU
	// phases or "render time" reads like saturation.
	presentWait time.Duration
}

var _ runtime.FrameRenderer = (*Renderer)(nil)

// NewRenderer creates a native streaming renderer for window using vsync to
// select its surface presentation mode.
func NewRenderer(
	window *glfw.Window,
	clear render.Color,
	manifest asset.Manifest,
	vsync VSyncMode,
	msaa render.MSAAMode,
	particleScale float64,
	wantGPUTiming bool,
	log *slog.Logger,
) (renderer *Renderer, err error) {
	if log == nil {
		log = slog.Default()
	}
	provider, err := fontatlas.New(manifest)
	if err != nil {
		return nil, fmt.Errorf("create font atlas provider: %w", err)
	}
	d, err := newDevice(window, vsync, wantGPUTiming, log)
	if err != nil {
		return nil, err
	}
	renderer = &Renderer{
		device:        d,
		clear:         clear,
		log:           log,
		fontProvider:  provider,
		textAtlases:   make(map[tess.AtlasKey]gpuTextAtlas),
		spriteBatches: make(map[render.SpriteBatchID]*nativeSpriteBatch),
		msaaSamples:   msaaSampleCount(msaa, d.formatFeatures, d.validateMSAASampleCount, log),
		particleScale: particleScale,
	}
	defer func() {
		if err != nil {
			renderer.Close()
			renderer = nil
		}
	}()

	if err = renderer.createPipelines(); err != nil {
		return nil, err
	}
	if err = renderer.createPostResources(); err != nil {
		log.Warn("native post-processing disabled", "err", err)
		renderer.postDisabled = true
	}
	if err = renderer.createSamplers(); err != nil {
		return nil, err
	}
	if err = renderer.createBuffers(); err != nil {
		return nil, err
	}
	resources := renderer.textureResources()
	renderer.textures, renderer.texSizes = buildTextures(manifest, resources, log)
	if err = renderer.rebuildSpriteBindGroups(); err != nil {
		return nil, err
	}
	renderer.particles, err = newGpuParticles(
		d.device, d.queue, manifest, renderer.textures, d.config.Format, renderer.msaaSamples, particleScale > 0,
	)
	if err != nil && renderer.particleScale > 0 {
		log.Warn("native particle offscreen pipelines unavailable, using direct rendering", "err", err)
		renderer.particleScale = 0
		renderer.particles, err = newGpuParticles(
			d.device, d.queue, manifest, renderer.textures, d.config.Format, renderer.msaaSamples, false,
		)
	}
	if err != nil {
		// Particle effects are optional presentation. Keep the native host usable
		// when a driver rejects the compute/indirect pipeline, matching the WASM
		// backend's best-effort particle path.
		log.Warn("native particle renderer disabled", "err", err)
		renderer.particles = nil
	}
	if renderer.particles != nil && renderer.particleScale > 0 {
		if err = renderer.createCompositePipeline(); err != nil {
			renderer.disableParticleOffscreen(err)
			err = nil
		}
	}
	if renderer.timing, err = newGPUTiming(d); err != nil {
		// The adapter granted the feature but its query resources would not
		// build. Telemetry is not worth failing a window over.
		log.Warn("native GPU timing disabled", "err", err)
		renderer.timing = nil
		err = nil
	}
	return renderer, nil
}

// createCompositePipeline builds the fullscreen-triangle pipeline that blends
// one particle offscreen target into the MSAA main pass with premultiplied
// source-over, plus the bind group layout and sampler its pooled targets use.
func (r *Renderer) createCompositePipeline() error {
	var err error
	if r.compositeLayout, err = r.device.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
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
	}); err != nil {
		return fmt.Errorf("create bind group layout: %w", err)
	}
	pipelineLayout, err := r.device.device.TryCreatePipelineLayout(
		&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{r.compositeLayout}},
	)
	if err != nil {
		return fmt.Errorf("create pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()
	module, err := r.device.device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "Arasy particle composite shader",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: shaders.ParticlesCompositeWGSL},
	})
	if err != nil {
		return fmt.Errorf("create shader: %w", err)
	}
	defer module.Release()
	r.compositePipeline, err = r.device.device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:       "Arasy particle composite pipeline",
		Layout:      pipelineLayout,
		Vertex:      wgpu.VertexState{Module: module, EntryPoint: "vs_main"},
		Primitive:   wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList},
		Multisample: wgpu.MultisampleState{Count: r.msaaSamples, Mask: ^uint32(0)},
		Fragment: &wgpu.FragmentState{
			Module:     module,
			EntryPoint: "fs_main",
			Targets: []wgpu.ColorTargetState{{
				Format: r.device.config.Format, Blend: premultipliedBlend(), WriteMask: wgpu.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("create pipeline: %w", err)
	}
	r.compositeSampler, err = r.device.device.TryCreateSampler(
		&wgpu.SamplerDescriptor{MagFilter: wgpu.FilterModeLinear, MinFilter: wgpu.FilterModeLinear, MaxAnisotropy: 1},
	)
	if err != nil {
		return fmt.Errorf("create sampler: %w", err)
	}
	return nil
}

func (r *Renderer) disableParticleOffscreen(err error) {
	r.log.Warn("native particle offscreen compositing disabled", "err", err)
	r.particleScale = 0
	r.releaseParticleTargets()
	releasePipeline(&r.compositePipeline)
	releaseSampler(&r.compositeSampler)
	if r.compositeLayout != nil {
		r.compositeLayout.Release()
		r.compositeLayout = nil
	}
}

// particleTarget is one pooled offscreen target plus its composite bind group.
type particleTarget struct {
	texture   *wgpu.Texture
	view      *wgpu.TextureView
	bindGroup *wgpu.BindGroup
}

// particleTargetDims returns the offscreen target size for the given surface
// dimensions and Config.ParticleOffscreenScale.
func particleTargetDims(w, h int, scale float64) (int, int) {
	return max(1, int(math.Ceil(float64(w)*scale))), max(1, int(math.Ceil(float64(h)*scale)))
}

// ensureParticleTargets grows the pooled offscreen targets to n and rebuilds
// the pool when the composite dimensions change. The pool never shrinks: its
// high-water mark is the largest number of interleaved particle groups a
// frame has produced.
func (r *Renderer) ensureParticleTargets(n, ow, oh int) error {
	if ow != r.particleTargetW || oh != r.particleTargetH {
		r.releaseParticleTargets()
		r.particleTargetW, r.particleTargetH = ow, oh
	}
	for len(r.particleTargets) < n {
		texture, err := r.device.device.TryCreateTexture(&wgpu.TextureDescriptor{
			Label:         "Arasy particle offscreen target",
			Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
			Dimension:     wgpu.TextureDimension2D,
			Size:          wgpu.Extent3D{Width: uint32(ow), Height: uint32(oh), DepthOrArrayLayers: 1},
			Format:        r.device.config.Format,
			MipLevelCount: 1,
			SampleCount:   1,
		})
		if err != nil {
			return fmt.Errorf("create particle offscreen target: %w", err)
		}
		view, err := texture.TryCreateView(nil)
		if err != nil {
			texture.Release()
			return fmt.Errorf("create particle offscreen view: %w", err)
		}
		bindGroup, err := r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
			Layout: r.compositeLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, TextureView: view},
				{Binding: 1, Sampler: r.compositeSampler},
			},
		})
		if err != nil {
			view.Release()
			texture.Release()
			return fmt.Errorf("create particle composite bind group: %w", err)
		}
		r.particleTargets = append(r.particleTargets, particleTarget{texture: texture, view: view, bindGroup: bindGroup})
	}
	return nil
}

func (r *Renderer) releaseParticleTargets() {
	for _, t := range r.particleTargets {
		if t.bindGroup != nil {
			t.bindGroup.Release()
		}
		releaseTextureView(&t.view)
		releaseTexture(&t.texture)
	}
	r.particleTargets = nil
}

// UpdateResources applies dynamic texture and retained sprite batch updates.
func (r *Renderer) UpdateResources(updates runtime.FrameUpdates) {
	if r == nil || r.err != nil {
		return
	}
	if err := r.textureResources().updateTextures(updates.Textures, r.textures); err != nil {
		r.recordErr(err)
		return
	}
	if len(updates.SpriteBatches) > 0 {
		if err := r.applySpriteBatchUpdates(updates.SpriteBatches); err != nil {
			r.recordErr(err)
			return
		}
	}
	if len(updates.TextBatches) > 0 {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "native-text-batches-unsupported"},
			"native renderer skips retained text batches")
	}
}

// Render draws one frame. Errors are retained for Err because FrameRenderer
// cannot return them to the host loop.
func (r *Renderer) Render(frame runtime.Frame) {
	if r == nil {
		return
	}
	if r.timeCPU {
		r.cpuFrame = cpuFrameTiming{}
		r.cpuPhasesSet = false
		r.presentWait = 0
	}
	if r.err != nil || r.device == nil || r.device.config == nil {
		return
	}
	width, height := int(r.device.config.Width), int(r.device.config.Height)
	if width <= 0 || height <= 0 {
		return
	}
	r.tess.SetSpriteArray(false)
	var phaseStart time.Time
	if r.timeCPU {
		phaseStart = time.Now()
	}
	runs := r.tess.Build(tess.Frame(frame), width, height, r.fontProvider, r.texSizes)
	layerCameras := tess.LayerCameras(frame.Camera, runs)
	solid, textured := r.tess.Streams()
	sprites := r.tess.SpriteStream()
	textGlyphs := r.tess.TextGlyphStream()
	textLabels := r.tess.TextLabelStream()
	if r.timeCPU {
		r.cpuTess = time.Since(phaseStart)
		phaseStart = time.Now()
	}
	uniformOffsets, err := r.prepareSpriteUniforms(runs, frame.Camera, layerCameras, width, height)
	if err != nil {
		r.recordErr(err)
		return
	}
	if err := r.uploadStreams(solid, textured, sprites, textGlyphs, textLabels); err != nil {
		r.recordErr(err)
		return
	}
	if r.timeCPU {
		r.cpuUpload = time.Since(phaseStart)
	}
	if err := r.ensureMSAATarget(width, height); err != nil {
		r.recordErr(err)
		return
	}
	var acquireStart time.Time
	if r.timeCPU {
		acquireStart = time.Now()
	}
	surfaceTexture, err := r.device.surface.TryGetCurrentTexture()
	if r.timeCPU {
		r.cpuFrame.acquireWait = time.Since(acquireStart)
		r.presentWait = r.cpuFrame.acquireWait
	}
	if err != nil {
		r.recordErr(fmt.Errorf("acquire surface texture: %w", err))
		return
	}
	if surfaceTexture.Texture != nil {
		defer surfaceTexture.Texture.Release()
	}
	switch decideSurfaceStatus(surfaceTexture.Status) {
	case actionSkip:
		return
	case actionReconfigure:
		r.device.configure(width, height)
		return
	case actionFatal:
		r.recordErr(fmt.Errorf("acquire surface texture: %s", surfaceTexture.Status))
		return
	case actionRender:
		if surfaceTexture.Texture == nil {
			r.recordErr(errors.New("acquire surface texture: success status without texture"))
			return
		}
	}
	surfaceView, err := surfaceTexture.Texture.TryCreateView(nil)
	if err != nil {
		r.recordErr(fmt.Errorf("create surface texture view: %w", err))
		return
	}
	defer surfaceView.Release()
	if r.timeCPU {
		phaseStart = time.Now()
	}
	var encodeErr error
	if chain, ok := lastNativePostChain(frame.Commands); ok && !r.postDisabled {
		encodeErr = r.encodePostFrame(runs, uniformOffsets, layerCameras, surfaceView, frame, chain, width, height)
		if encodeErr != nil {
			r.disablePost(encodeErr)
			encodeErr = r.encodeFrame(runs, uniformOffsets, layerCameras, surfaceView, frame.Camera, width, height)
		}
	} else {
		encodeErr = r.encodeFrame(runs, uniformOffsets, layerCameras, surfaceView, frame.Camera, width, height)
	}
	if encodeErr != nil {
		r.recordErr(encodeErr)
		return
	}
	if r.timeCPU {
		r.cpuFrame.cpuEndAt = time.Now()
		r.cpuEncode = r.cpuFrame.cpuEndAt.Sub(phaseStart)
		r.cpuPhasesSet = true
		r.cpuFrame.valid = !r.cpuFrame.submittedAt.IsZero()
	}
	if r.timeCPU {
		phaseStart = r.cpuFrame.cpuEndAt
	}
	r.device.surface.Present()
	if r.timeCPU {
		r.presentWait += time.Since(phaseStart)
	}
	r.presentedFrame = true
	if r.particles != nil {
		r.particles.endFrame()
	}
}

func (r *Renderer) hasPresentedFrame() bool {
	return r != nil && r.presentedFrame
}

// SetCPUTiming enables CPU-side phase instrumentation.
func (r *Renderer) SetCPUTiming(enabled bool) {
	if r != nil {
		r.timeCPU = enabled
	}
}

// SetGPUTiming enables per-pass timestamp sampling. It only has an effect when
// the device was created with the timestamp-query feature, which NewRenderer
// requests from its own wantGPUTiming argument — the feature has to be asked
// for at device creation, so this cannot turn timing on after the fact.
func (r *Renderer) SetGPUTiming(enabled bool) {
	if r != nil {
		r.timeGPU = enabled && r.timing != nil
	}
}

// particleGPUTimings reports the most recent timestamp-query sample: the
// particle emit pass, the simulate pass (including the pass boundary before
// it) and the render interval. Frames without particle commands still sample
// the render pass and report zero for the compute stages. ok is false until
// the first sample resolves, and always on devices without timestamp-query.
func (r *Renderer) particleGPUTimings() (prepare, simulate, render time.Duration, ok bool) {
	if r == nil || !r.timeGPU {
		return 0, 0, 0, false
	}
	return r.timing.timings()
}

// presentWaitDuration reports how long the last frame blocked on the
// presentation queue: surface acquire plus present. With vsync that is the
// vblank wait — idle time, not CPU work. ok is false while CPU timing is
// disabled, meaning no measurement was taken.
func (r *Renderer) presentWaitDuration() (time.Duration, bool) {
	if r == nil {
		return 0, false
	}
	return r.presentWait, r.timeCPU && r.cpuFrame.valid
}

// frameCPUTiming reports the boundaries of the most recently submitted frame.
// It stays invalid for a skipped/reconfigured surface so callers never reuse a
// previous frame's submission timestamp.
func (r *Renderer) frameCPUTiming() cpuFrameTiming {
	if r == nil || !r.timeCPU {
		return cpuFrameTiming{}
	}
	return r.cpuFrame
}

// adapterIDs returns the PCI vendor and device IDs of the adapter the
// renderer selected, for the OS probes that need to find the same GPU.
func (r *Renderer) adapterIDs() (vendor, device uint32) {
	if r == nil || r.device == nil {
		return 0, 0
	}
	return r.device.vendorID, r.device.deviceID
}

// renderCPUTimings reports CPU-side wall times for tessellation, upload and
// command encoding. These values exclude GPU execution time.
func (r *Renderer) renderCPUTimings() (time.Duration, time.Duration, time.Duration, bool) {
	if r == nil {
		return 0, 0, 0, false
	}
	return r.cpuTess, r.cpuUpload, r.cpuEncode, r.cpuPhasesSet
}

// particleGPUBufferBytes reports resident particle buffer allocations. A nil
// particle renderer means this metric is unavailable.
func (r *Renderer) particleGPUBufferBytes() (uint64, bool) {
	if r == nil || r.particles == nil {
		return 0, false
	}
	return r.particles.bufferBytes(), true
}

// Resize updates the native surface from physical framebuffer dimensions.
func (r *Renderer) Resize(fbW, fbH int) {
	if r == nil || r.err != nil {
		return
	}
	r.device.configure(fbW, fbH)
}

// Err returns the first render failure observed by the native host.
func (r *Renderer) Err() error {
	if r == nil {
		return nil
	}
	return r.err
}

// Close releases GPU resources and is safe to call more than once.
func (r *Renderer) Close() {
	if r == nil {
		return
	}
	for _, atlas := range r.textAtlases {
		atlas.release()
	}
	r.textAtlases = nil
	if r.particles != nil {
		r.particles.close()
		r.particles = nil
	}
	for id, batch := range r.spriteBatches {
		batch.release()
		delete(r.spriteBatches, id)
	}
	r.spriteBatches = nil
	for _, texture := range r.textures {
		texture.release()
	}
	r.textures = nil
	if r.timing != nil {
		r.timing.release()
		r.timing = nil
	}
	r.releaseParticleTargets()
	if r.compositeLayout != nil {
		r.compositeLayout.Release()
		r.compositeLayout = nil
	}
	releasePipeline(&r.compositePipeline)
	releaseSampler(&r.compositeSampler)
	releaseTextureView(&r.msaaView)
	releaseTexture(&r.msaaTexture)
	releaseBuffer(&r.solidBuffer)
	releaseBuffer(&r.texBuffer)
	releaseBuffer(&r.spriteBuffer)
	releaseBuffer(&r.textGlyphBuffer)
	releaseBuffer(&r.textLabelBuffer)
	releaseBuffer(&r.spriteUniform)
	releasePipeline(&r.solidPipeline)
	releasePipeline(&r.texturedPipeline)
	releasePipeline(&r.spritePipeline)
	releasePipeline(&r.textGlyphPipeline)
	releasePipeline(&r.solidAddPipeline)
	releasePipeline(&r.texturedAddPipeline)
	releasePipeline(&r.spriteAddPipeline)
	releasePipeline(&r.textGlyphAddPipeline)
	releasePipeline(&r.mipPipeline)
	if r.spritePipelineLayout != nil {
		r.spritePipelineLayout.Release()
		r.spritePipelineLayout = nil
	}
	if r.textGlyphPipelineLayout != nil {
		r.textGlyphPipelineLayout.Release()
		r.textGlyphPipelineLayout = nil
	}
	for _, pipeline := range r.postPipelines {
		releasePipeline(&pipeline)
	}
	r.postPipelines = nil
	for index := range r.postUniforms {
		releaseBuffer(&r.postUniforms[index])
	}
	r.postUniforms = nil
	r.releasePostBindGroups()
	r.releasePostTarget(&r.postSceneA)
	r.releasePostTarget(&r.postSceneB)
	for index := range r.postBloom {
		r.releasePostTarget(&r.postBloom[index])
	}
	r.postBloom = nil
	releaseSampler(&r.postSampler)

	releaseSampler(&r.textureSampler)
	releaseSampler(&r.mipSampler)
	if r.device != nil {
		r.device.destroy()
		r.device = nil
	}
}

func (r *Renderer) createPipelines() error {
	var err error
	if r.spritePipelineLayout, err = r.createSpritePipelineLayout(); err != nil {
		return err
	}
	if r.textGlyphPipelineLayout, err = r.createTextGlyphPipelineLayout(); err != nil {
		return err
	}
	if r.solidPipeline, err = r.createPipeline(
		"solid",
		shaders.SolidWGSL,
		r.device.config.Format,
		r.msaaSamples,
		solidVertexLayout(),
		alphaBlend(),
		nil,
	); err != nil {
		return err
	}

	if r.texturedPipeline, err = r.createPipeline(
		"textured",
		shaders.TexturedWGSL,
		r.device.config.Format,
		r.msaaSamples,
		texturedVertexLayout(),
		premultipliedBlend(),
		nil,
	); err != nil {
		return err
	}

	if r.spritePipeline, err = r.createPipeline(
		"sprite",
		shaders.SpriteWGSL,
		r.device.config.Format,
		r.msaaSamples,
		nil,
		premultipliedBlend(),
		r.spritePipelineLayout,
	); err != nil {
		return err
	}

	if r.textGlyphPipeline, err = r.createPipeline(
		"text glyph",
		shaders.TextGlyphWGSL,
		r.device.config.Format,
		r.msaaSamples,
		nil,
		alphaBlend(),
		r.textGlyphPipelineLayout,
	); err != nil {
		return err
	}

	if r.solidAddPipeline, err = r.createPipeline(
		"solid additive",
		shaders.SolidWGSL,
		r.device.config.Format,
		r.msaaSamples,
		solidVertexLayout(),
		additiveStraightBlend(),
		nil,
	); err != nil {
		return err
	}

	if r.texturedAddPipeline, err = r.createPipeline(
		"textured additive",
		shaders.TexturedWGSL,
		r.device.config.Format,
		r.msaaSamples,
		texturedVertexLayout(),
		additiveBlend(),
		nil,
	); err != nil {
		return err
	}

	if r.spriteAddPipeline, err = r.createPipeline(
		"sprite additive",
		shaders.SpriteWGSL,
		r.device.config.Format,
		r.msaaSamples,
		nil,
		additiveBlend(),
		r.spritePipelineLayout,
	); err != nil {
		return err
	}

	if r.textGlyphAddPipeline, err = r.createPipeline(
		"text glyph additive",
		shaders.TextGlyphWGSL,
		r.device.config.Format,
		r.msaaSamples,
		nil,
		additiveStraightBlend(),
		r.textGlyphPipelineLayout,
	); err != nil {
		return err
	}

	r.mipPipeline, err = r.createPipeline("mipmap", shaders.MipmapWGSL, wgpu.TextureFormatRGBA8Unorm, 1, nil, nil, nil)
	return err
}

func (r *Renderer) createSpritePipelineLayout() (*wgpu.PipelineLayout, error) {
	layout, err := r.device.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{
				SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D,
			}},
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{
				Type: wgpu.SamplerBindingTypeFiltering,
			}},
			{Binding: 2, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeReadOnlyStorage,
			}},
			{Binding: 3, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeUniform, HasDynamicOffset: true, MinBindingSize: tess.SpriteUniformSize,
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create sprite bind group layout: %w", err)
	}
	pipelineLayout, err := r.device.device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{layout},
	})
	layout.Release()
	if err != nil {
		return nil, fmt.Errorf("create sprite pipeline layout: %w", err)
	}
	return pipelineLayout, nil
}

func (r *Renderer) createTextGlyphPipelineLayout() (*wgpu.PipelineLayout, error) {
	layout, err := r.device.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{
				SampleType: wgpu.TextureSampleTypeFloat, ViewDimension: wgpu.TextureViewDimension2D,
			}},
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{
				Type: wgpu.SamplerBindingTypeFiltering,
			}},
			{Binding: 2, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeReadOnlyStorage,
			}},
			{Binding: 3, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeReadOnlyStorage,
			}},
			{Binding: 4, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeReadOnlyStorage,
			}},
			{Binding: 5, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeUniform, HasDynamicOffset: true, MinBindingSize: tess.SpriteUniformSize,
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create text bind group layout: %w", err)
	}
	pipelineLayout, err := r.device.device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{layout},
	})
	layout.Release()
	if err != nil {
		return nil, fmt.Errorf("create text pipeline layout: %w", err)
	}
	return pipelineLayout, nil
}

func (r *Renderer) createPipeline(
	label, source string,
	format wgpu.TextureFormat,
	samples uint32,
	buffers []wgpu.VertexBufferLayout,
	blend *wgpu.BlendState,
	layout *wgpu.PipelineLayout,
) (*wgpu.RenderPipeline, error) {
	return r.createPipelineEntry(label, source, "fs_main", format, samples, buffers, blend, layout)
}

func (r *Renderer) createPipelineEntry(
	label, source, fragmentEntry string,
	format wgpu.TextureFormat,
	samples uint32,
	buffers []wgpu.VertexBufferLayout,
	blend *wgpu.BlendState,
	layout *wgpu.PipelineLayout,
) (*wgpu.RenderPipeline, error) {
	module, err := r.device.device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "Arasy " + label + " shader",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: source},
	})
	if err != nil {
		return nil, fmt.Errorf("create %s shader: %w", label, err)
	}
	defer module.Release()
	pipeline, err := r.device.device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:       "Arasy " + label + " pipeline",
		Layout:      layout,
		Vertex:      wgpu.VertexState{Module: module, EntryPoint: "vs_main", Buffers: buffers},
		Primitive:   wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList},
		Multisample: wgpu.MultisampleState{Count: samples, Mask: ^uint32(0)},
		Fragment: &wgpu.FragmentState{Module: module, EntryPoint: fragmentEntry, Targets: []wgpu.ColorTargetState{{
			Format: format, Blend: blend, WriteMask: wgpu.ColorWriteMaskAll,
		}}},
	})
	if err != nil {
		return nil, fmt.Errorf("create %s pipeline: %w", label, err)
	}
	return pipeline, nil
}

func (r *Renderer) createSamplers() error {
	textureDesc := textureSamplerDescriptor()

	var err error
	r.textureSampler, err = r.device.device.TryCreateSampler(&textureDesc)
	if err != nil {
		return fmt.Errorf("create texture sampler: %w", err)
	}
	mipDesc := mipSamplerDescriptor()

	r.mipSampler, err = r.device.device.TryCreateSampler(&mipDesc)
	if err != nil {
		return fmt.Errorf("create mip sampler: %w", err)
	}
	return nil
}

func (r *Renderer) createBuffers() error {
	var err error
	if r.solidBuffer, err = r.newBuffer(
		"solid vertices",
		gpubuffer.InitialCapacity,
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.solidCap = gpubuffer.InitialCapacity

	if r.texBuffer, err = r.newBuffer(
		"textured vertices",
		gpubuffer.InitialCapacity,
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.texCap = gpubuffer.InitialCapacity

	if r.spriteBuffer, err = r.newBuffer(
		"sprite records",
		gpubuffer.InitialCapacity,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.spriteCap = gpubuffer.InitialCapacity

	if r.textGlyphBuffer, err = r.newBuffer(
		"text glyph records",
		gpubuffer.InitialCapacity,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.textGlyphCap = gpubuffer.InitialCapacity

	if r.textLabelBuffer, err = r.newBuffer(
		"text label records",
		gpubuffer.InitialCapacity,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.textLabelCap = gpubuffer.InitialCapacity
	r.spriteUniformSlotSize = uniformSlotSize(tess.SpriteUniformSize, r.device.device.GetLimits().MinUniformBufferOffsetAlignment)
	r.spriteUniformCap = 64
	r.spriteUniform, err = r.newBuffer(
		"sprite uniforms",
		r.spriteUniformSlotSize*r.spriteUniformCap,
		wgpu.BufferUsageUniform|wgpu.BufferUsageCopyDst,
	)
	return err
}

func uniformSlotSize(size int, alignment uint32) int {
	if alignment == 0 {
		return size
	}
	a := int(alignment)
	return (size + a - 1) / a * a
}

func (r *Renderer) ensureSpriteUniformCapacity(required int) error {
	if required <= r.spriteUniformCap {
		return nil
	}
	next := gpubuffer.GrowCapacity(r.spriteUniformCap, required)
	buffer, err := r.newBuffer(
		"sprite uniforms",
		r.spriteUniformSlotSize*next,
		wgpu.BufferUsageUniform|wgpu.BufferUsageCopyDst,
	)
	if err != nil {
		return err
	}
	old := r.spriteUniform
	r.spriteUniform, r.spriteUniformCap = buffer, next
	if old != nil {
		old.Release()
	}
	if err := r.rebuildSpriteBindGroups(); err != nil {
		return err
	}
	if err := r.rebuildTextGlyphBindGroups(); err != nil {
		return err
	}
	for _, batch := range r.spriteBatches {
		if err := r.rebuildSpriteBatchBindGroups(batch); err != nil {
			return err
		}
	}
	return nil
}

func (r *Renderer) newBuffer(label string, size int, usage wgpu.BufferUsage) (*wgpu.Buffer, error) {
	buffer, err := r.device.device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Label: "Arasy " + label,
		Size:  uint64(size),
		Usage: usage,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s buffer: %w", label, err)
	}
	return buffer, nil
}

func (r *Renderer) uploadStreams(
	solid, textured, sprites, glyphs, labels []byte,
) error {
	if err := r.ensureBuffers(len(solid), len(textured), len(sprites), len(glyphs), len(labels)); err != nil {
		return err
	}
	for _, upload := range []struct {
		buffer *wgpu.Buffer
		data   []byte
	}{
		{r.solidBuffer, solid},
		{r.texBuffer, textured},
		{r.spriteBuffer, sprites},
		{r.textGlyphBuffer, glyphs},
		{r.textLabelBuffer, labels},
	} {
		if len(upload.data) > 0 {
			if err := r.device.queue.TryWriteBuffer(upload.buffer, 0, upload.data); err != nil {
				return fmt.Errorf("upload stream: %w", err)
			}
		}
	}
	return nil
}

// noSpriteUniformEntry marks runs that draw without a camera uniform, so
// prepareSpriteUniforms can map entry indices to byte offsets in one pass
// without recomputing run keys.
const noSpriteUniformEntry = ^uint32(0)

// Uniform entries are keyed by the camera data they pack, not by draw run.
// The key logic is shared with the wasm backend and lives in tess.
type nativeUniformEntry struct {
	key    tess.UniformKey
	offset uint32
}

func nativeUniformEntryIndex(entries []nativeUniformEntry, key tess.UniformKey) int {
	for index, entry := range entries {
		if entry.key == key {
			return index
		}
	}
	return -1
}

func (r *Renderer) prepareSpriteUniforms(runs []tess.StreamRun, base render.Camera, cameras []tess.LayerCamera, width, height int) ([]uint32, error) {
	// A camera upload is shared by every run that uses the same packed view.
	if cap(r.spriteUniformOffsets) < len(runs) {
		r.spriteUniformOffsets = make([]uint32, len(runs))
	} else {
		r.spriteUniformOffsets = r.spriteUniformOffsets[:len(runs)]
	}
	offsets := r.spriteUniformOffsets
	r.spriteUniformEntries = r.spriteUniformEntries[:0]
	for index, run := range runs {
		offsets[index] = noSpriteUniformEntry
		key, ok := tess.UniformKeyForKind(run.Kind, run.Layer, base, cameras)
		if !ok {
			continue
		}
		entryIndex := nativeUniformEntryIndex(r.spriteUniformEntries, key)
		if entryIndex < 0 {
			r.spriteUniformEntries = append(r.spriteUniformEntries, nativeUniformEntry{key: key})
			entryIndex = len(r.spriteUniformEntries) - 1
		}
		offsets[index] = uint32(entryIndex)
	}
	if len(r.spriteUniformEntries) == 0 {
		clear(offsets)
		return offsets, nil
	}
	if err := r.ensureSpriteUniformCapacity(len(r.spriteUniformEntries)); err != nil {
		return nil, err
	}
	var packed [tess.SpriteUniformSize]byte
	for entryIndex := range r.spriteUniformEntries {
		entry := &r.spriteUniformEntries[entryIndex]
		tess.PackUniform(packed[:], entry.key, base, cameras, width, height)
		offset := uint32(entryIndex * r.spriteUniformSlotSize)
		entry.offset = offset
		if err := r.device.queue.TryWriteBuffer(r.spriteUniform, uint64(offset), packed[:]); err != nil {
			return nil, fmt.Errorf("upload sprite uniform: %w", err)
		}
	}
	for index := range offsets {
		if offsets[index] == noSpriteUniformEntry {
			offsets[index] = 0
			continue
		}
		offsets[index] = r.spriteUniformEntries[offsets[index]].offset
	}
	return offsets, nil
}

func (r *Renderer) ensureBuffers(solid, textured, sprites, glyphs, labels int) error {
	spriteBufferChanged := sprites > r.spriteCap
	textBuffersChanged := glyphs > r.textGlyphCap || labels > r.textLabelCap
	var err error
	if r.solidBuffer, r.solidCap, err = r.growBuffer(
		r.solidBuffer,
		r.solidCap,
		solid,
		"solid vertices",
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}

	if r.texBuffer, r.texCap, err = r.growBuffer(
		r.texBuffer,
		r.texCap,
		textured,
		"textured vertices",
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}

	if r.spriteBuffer, r.spriteCap, err = r.growBuffer(
		r.spriteBuffer,
		r.spriteCap,
		sprites,
		"sprite records",
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}

	if r.textGlyphBuffer, r.textGlyphCap, err = r.growBuffer(
		r.textGlyphBuffer,
		r.textGlyphCap,
		glyphs,
		"text glyph records",
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}

	if r.textLabelBuffer, r.textLabelCap, err = r.growBuffer(
		r.textLabelBuffer,
		r.textLabelCap,
		labels,
		"text label records",
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}

	if spriteBufferChanged {
		if err := r.rebuildSpriteBindGroups(); err != nil {
			return err
		}
	}
	if textBuffersChanged {
		return r.rebuildTextGlyphBindGroups()
	}
	return nil
}

func (r *Renderer) growBuffer(
	buffer *wgpu.Buffer,
	current,
	required int,
	label string,
	usage wgpu.BufferUsage,
) (*wgpu.Buffer, int, error) {
	if required <= current {
		return buffer, current, nil
	}
	next := gpubuffer.GrowCapacity(current, required)
	replacement, err := r.newBuffer(label, next, usage)
	if err != nil {
		return nil, 0, err
	}
	if buffer != nil {
		buffer.Release()
	}
	return replacement, next, nil
}

func (r *Renderer) rebuildSpriteBindGroups() error {
	layout := r.spritePipeline.GetBindGroupLayout(0)
	defer layout.Release()
	for i := range r.textures {
		texture := &r.textures[i]
		if texture.view == nil {
			continue
		}
		if texture.spriteBindGroup != nil {
			texture.spriteBindGroup.Release()
		}
		bindGroup, err := r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "Arasy sprite bind group",
			Layout: layout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, TextureView: texture.view},
				{Binding: 1, Sampler: r.textureSampler},
				{Binding: 2, Buffer: r.spriteBuffer, Size: wgpu.WholeSize},
				{Binding: 3, Buffer: r.spriteUniform, Size: tess.SpriteUniformSize},
			},
		})
		if err != nil {
			return fmt.Errorf("create sprite bind group: %w", err)
		}
		texture.spriteBindGroup = bindGroup
	}
	return nil
}

func (r *Renderer) rebuildTextGlyphBindGroups() error {
	resources := r.textAtlasResources()
	for key, atlas := range r.textAtlases {
		bindGroup, err := resources.createTextGlyphBindGroup(atlas.view, atlas.sampler, atlas.metricBuffer)
		if err != nil {
			return err
		}
		if atlas.glyphBindGroup != nil {
			atlas.glyphBindGroup.Release()
		}
		atlas.glyphBindGroup = bindGroup
		r.textAtlases[key] = atlas
	}
	return nil
}

func (r *Renderer) ensureMSAATarget(width, height int) error {
	if r.msaaSamples <= 1 {
		// No multi-sampling: the pass renders straight to the surface, so
		// free any target a previous configuration allocated.
		releaseTextureView(&r.msaaView)
		releaseTexture(&r.msaaTexture)
		r.msaaWidth, r.msaaHeight = 0, 0
		return nil
	}
	if r.msaaTexture != nil && r.msaaWidth == width && r.msaaHeight == height {
		return nil
	}
	releaseTextureView(&r.msaaView)
	releaseTexture(&r.msaaTexture)
	texture, err := r.device.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "Arasy MSAA target",
		Usage:         wgpu.TextureUsageRenderAttachment,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		Format:        r.device.config.Format,
		MipLevelCount: 1,
		SampleCount:   r.msaaSamples,
	})
	if err != nil {
		return fmt.Errorf("create MSAA target: %w", err)
	}
	view, err := texture.TryCreateView(nil)
	if err != nil {
		texture.Release()
		return fmt.Errorf("create MSAA view: %w", err)
	}
	r.msaaTexture, r.msaaView, r.msaaWidth, r.msaaHeight = texture, view, width, height
	return nil
}

func (r *Renderer) encodeFrame(
	runs []tess.StreamRun,
	uniformOffsets []uint32,
	layerCameras []tess.LayerCamera,
	surfaceView *wgpu.TextureView,
	camera render.Camera,
	width, height int,
) error {
	encoder, err := r.device.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("create render command encoder: %w", err)
	}
	defer encoder.Release()
	var groupOf []int
	offscreenParticles := false
	// timedCompute records whether the particle compute passes ran, which
	// decides how much of the query set is safe to resolve: a pass that never
	// ran wrote no timestamp, and resolving one that was never written loses
	// the device on Vulkan.
	sampling := r.timeGPU && r.timing.sampleFrame()
	timedCompute := false
	if r.particles != nil {
		sims := make([]particleSim, 0)
		for _, run := range runs {
			if run.Kind != tess.DrawRunParticles {
				continue
			}
			if sim, ok := r.particles.prepare(run.Particles, tess.CameraForLayer(camera, layerCameras, run.Layer), width, height); ok {
				sims = append(sims, sim)
			}
		}
		var emitWrites, simulateWrites *wgpu.PassTimestampWrites
		if sampling && len(sims) > 0 {
			emitWrites = r.timing.prepareWrites()
			simulateWrites = r.timing.simulateWrites()
			timedCompute = true
		}
		if err := r.particles.encode(encoder, sims, emitWrites, simulateWrites); err != nil {
			return fmt.Errorf("encode particle simulation: %w", err)
		}

		// Contiguous particle runs form composite groups: with offscreen
		// compositing enabled, each group renders into one pooled target and
		// is composited once, at the painter-order position of its first run.
		var groups [][]nativeParticleRun
		groups, groupOf = groupParticleRuns(runs, camera, layerCameras)
		offscreenParticles = r.particleScale > 0 && len(groups) > 0
		if offscreenParticles {
			ow, oh := particleTargetDims(width, height, r.particleScale)
			if err := r.ensureParticleTargets(len(groups), ow, oh); err != nil {
				r.disableParticleOffscreen(err)
				offscreenParticles = false
				groupOf = nil
			} else {
				var beginWrites *wgpu.PassTimestampWrites
				if sampling {
					beginWrites = r.timing.renderBeginWrites()
				}
				if err := r.encodeParticleOffscreen(encoder, groups, ow, oh, beginWrites); err != nil {
					return err
				}
			}
		}
	}
	attachment := wgpu.RenderPassColorAttachment{
		View:          r.msaaView,
		ResolveTarget: surfaceView,
		LoadOp:        wgpu.LoadOpClear,
		StoreOp:       wgpu.StoreOpDiscard,
		ClearValue:    clearColor(r.clear),
	}
	if r.msaaSamples <= 1 {
		// No multi-sampling: draw straight into the surface and keep the
		// result — there is no resolve target to carry it for us.
		attachment.View = surfaceView
		attachment.ResolveTarget = nil
		attachment.StoreOp = wgpu.StoreOpStore
	}
	// With offscreen compositing the interval already opened on the first
	// offscreen pass, so the main pass only closes it.
	var mainWrites *wgpu.PassTimestampWrites
	if sampling {
		if offscreenParticles {
			mainWrites = r.timing.renderEndWrites()
		} else {
			mainWrites = r.timing.renderWrites()
		}
	}
	pass, err := encoder.TryBeginRenderPass(&wgpu.RenderPassDescriptor{
		Label:            "Arasy render pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{attachment},
		TimestampWrites:  mainWrites,
	})
	if err != nil {
		return fmt.Errorf("begin render pass: %w", err)
	}
	defer pass.Release()
	lastComposited := -1
	for i, run := range runs {
		group := -1
		if groupOf != nil {
			group = groupOf[i]
		}
		effective := tess.CameraForLayer(camera, layerCameras, run.Layer)
		if err := r.drawRun(pass, run, uniformOffsets[i], effective, width, height, group, offscreenParticles, &lastComposited); err != nil {
			return err
		}
	}
	if err := pass.TryEnd(); err != nil {
		return fmt.Errorf("end render pass: %w", err)
	}
	slot := -1
	if sampling {
		slot = r.timing.encodeResolve(encoder, timedCompute)
	}
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish render command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.device.queue.Submit(commandBuffer)
	if r.timeCPU {
		r.cpuFrame.submittedAt = time.Now()
	}
	if r.timeGPU {
		// Map after submit, pump whatever completed since the last frame, then
		// read the slots those callbacks marked. None of it blocks: the sample
		// lands a frame or more later — which is why the pumping runs on every
		// frame and not only on the ones that sampled.
		r.timing.readback(slot)
		r.timing.poll()
		r.timing.drain()
	}
	return nil
}

// groupParticleRuns assigns each contiguous run of tess.DrawRunParticles
// entries in runs to one composite group, mirroring the WASM backend's
// grouping (platform/wasm/webgpu.go). groupOf[i] is -1 for non-particle runs.
type nativeParticleRun struct {
	command render.DrawParticles
	camera  render.Camera
}

func groupParticleRuns(runs []tess.StreamRun, base render.Camera, cameras []tess.LayerCamera) (groups [][]nativeParticleRun, groupOf []int) {
	groupOf = make([]int, len(runs))
	inGroup := false
	for i, run := range runs {
		if run.Kind != tess.DrawRunParticles {
			groupOf[i] = -1
			inGroup = false
			continue
		}
		if !inGroup {
			groups = append(groups, nil)
			inGroup = true
		}
		idx := len(groups) - 1
		groupOf[i] = idx
		groups[idx] = append(groups[idx], nativeParticleRun{
			command: run.Particles,
			camera:  tess.CameraForLayer(base, cameras, run.Layer),
		})
	}
	return groups, groupOf
}

// encodeParticleOffscreen records one single-sample render pass per composite
// group, drawing its particle runs into a pooled target cleared to
// transparent black. Painter order inside the group is preserved; ordering
// against the rest of the scene is restored by compositing at the group's
// position in the main pass.
// beginWrites, when non-nil, opens the render timing interval on the first
// offscreen pass so the measurement covers the composites and the gaps between
// passes, not just the main pass.
func (r *Renderer) encodeParticleOffscreen(
	encoder *wgpu.CommandEncoder,
	groups [][]nativeParticleRun,
	ow, oh int,
	beginWrites *wgpu.PassTimestampWrites,
) error {
	for gi, cmds := range groups {
		var writes *wgpu.PassTimestampWrites
		if gi == 0 {
			writes = beginWrites
		}
		pass, err := encoder.TryBeginRenderPass(&wgpu.RenderPassDescriptor{
			Label: "Arasy particle offscreen pass",
			ColorAttachments: []wgpu.RenderPassColorAttachment{{
				View:       r.particleTargets[gi].view,
				LoadOp:     wgpu.LoadOpClear,
				StoreOp:    wgpu.StoreOpStore,
				ClearValue: wgpu.Color{},
			}},
			TimestampWrites: writes,
		})
		if err != nil {
			return fmt.Errorf("begin particle offscreen pass: %w", err)
		}
		for _, run := range cmds {
			r.particles.drawTo(pass, run.command, run.camera, r.particleScale, ow, oh, true)
		}
		if err := pass.TryEnd(); err != nil {
			pass.Release()
			return fmt.Errorf("end particle offscreen pass: %w", err)
		}
		pass.Release()
	}
	return nil
}

func (r *Renderer) drawRun(
	pass *wgpu.RenderPassEncoder,
	run tess.StreamRun,
	uniformOffset uint32,
	camera render.Camera,
	width, height int,
	group int,
	offscreenParticles bool,
	lastComposited *int,
) error {
	additive := run.Blend == render.BlendAdditive
	switch run.Kind {
	case tess.DrawRunSolid:
		pass.SetPipeline(pick(additive, r.solidAddPipeline, r.solidPipeline))
		pass.SetVertexBuffer(0, r.solidBuffer, 0, uint64(r.solidCap))
		pass.Draw(uint32(run.VertexCount), 1, uint32(run.ByteOffset/tess.SolidVertexStride), 0)
	case tess.DrawRunTextured:
		texture, ok := textureFor(r.textures, run.Texture)
		if !ok {
			return nil
		}
		pass.SetPipeline(pick(additive, r.texturedAddPipeline, r.texturedPipeline))
		pass.SetVertexBuffer(0, r.texBuffer, 0, uint64(r.texCap))
		pass.SetBindGroup(0, texture.bindGroup, nil)
		pass.Draw(uint32(run.VertexCount), 1, uint32(run.ByteOffset/tess.TexVertexStride), 0)
	case tess.DrawRunSprites:
		texture, ok := textureFor(r.textures, run.Texture)
		if !ok || texture.spriteBindGroup == nil {
			return nil
		}
		pass.SetPipeline(pick(additive, r.spriteAddPipeline, r.spritePipeline))
		pass.SetBindGroup(0, texture.spriteBindGroup, []uint32{uniformOffset})
		pass.Draw(uint32(run.VertexCount*6), 1, uint32(run.ByteOffset/tess.SpriteStride*6), 0)
	case tess.DrawRunText:
		atlas, ok, err := r.textAtlasResources().textAtlasFor(r.textAtlases, r.fontProvider, run.AtlasKey)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		pass.SetPipeline(pick(additive, r.textGlyphAddPipeline, r.textGlyphPipeline))
		pass.SetBindGroup(0, atlas.glyphBindGroup, []uint32{uniformOffset})
		pass.Draw(uint32(run.VertexCount*6), 1, uint32(run.ByteOffset/tess.TextGlyphStride*6), 0)
	case tess.DrawRunParticles:
		if r.particles == nil {
			return nil
		}
		if !offscreenParticles {
			r.particles.draw(pass, run.Particles, camera, width, height)
			return nil
		}
		if group == *lastComposited {
			return nil
		}
		*lastComposited = group
		pass.SetPipeline(r.compositePipeline)
		pass.SetBindGroup(0, r.particleTargets[group].bindGroup, nil)
		pass.Draw(3, 1, 0, 0)
	case tess.DrawRunSpriteBatch:
		batch, ok := r.spriteBatches[run.SpriteBatch.Batch]
		if !ok {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "native-sprite-batch-missing", ID: uint64(run.SpriteBatch.Batch)},
				"native: DrawSpriteBatch references a missing or deleted batch; command skipped")
			return nil
		}
		if batch.count == 0 {
			return nil
		}
		x, y, sw, sh, visible := particlegpu.ScissorRect(run.SpriteBatch.Clip, camera, 1, width, height)
		pass.SetScissorRect(uint32(x), uint32(y), uint32(sw), uint32(sh))
		if visible {
			for _, rg := range batch.ranges {
				if rg.bindGroup == nil || rg.count == 0 {
					continue
				}
				pass.SetPipeline(pick(additive, r.spriteAddPipeline, r.spritePipeline))
				pass.SetBindGroup(0, rg.bindGroup, []uint32{uniformOffset})
				pass.Draw(uint32(rg.count*6), 1, uint32(rg.start*6), 0)
			}
		}
		if run.SpriteBatch.Clip.Width() > 0 && run.SpriteBatch.Clip.Height() > 0 {
			pass.SetScissorRect(0, 0, uint32(width), uint32(height))
		}
	case tess.DrawRunTextBatch:
		r.warnSkipped("text batches")
	}
	return nil
}

func (r *Renderer) textureResources() textureResources {
	return textureResources{
		device:           r.device.device,
		queue:            r.device.queue,
		texturedPipeline: r.texturedPipeline,
		mipPipeline:      r.mipPipeline,
		textureSampler:   r.textureSampler,
		mipSampler:       r.mipSampler,
	}
}

func (r *Renderer) textAtlasResources() textAtlasResources {
	return textAtlasResources{
		device:            r.device.device,
		queue:             r.device.queue,
		textGlyphPipeline: r.textGlyphPipeline,
		glyphBuffer:       r.textGlyphBuffer,
		labelBuffer:       r.textLabelBuffer,
		uniformBuffer:     r.spriteUniform,
	}
}

func (r *Renderer) recordErr(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *Renderer) warnSkipped(kind string) {
	r.diagnostics.Log(
		r.log, slog.LevelWarn,
		diagnostic.Key{Code: "native-" + kind + "-unsupported"},
		"native renderer skips unsupported streaming run",
		"kind", kind,
	)
}
func textureFor(textures []gpuTexture, id render.TextureID) (gpuTexture, bool) {
	if id == 0 || int(id) >= len(textures) || textures[id].bindGroup == nil {
		return gpuTexture{}, false
	}
	return textures[id], true
}
func clearColor(color render.Color) wgpu.Color {
	return wgpu.Color{R: color.R * color.A, G: color.G * color.A, B: color.B * color.A, A: 1}
}
func alphaBlend() *wgpu.BlendState {
	return &wgpu.BlendState{
		Color: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorSrcAlpha,
			DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
		},
		Alpha: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorOne,
			DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
		},
	}
}
func premultipliedBlend() *wgpu.BlendState {
	return &wgpu.BlendState{Color: wgpu.BlendComponent{
		Operation: wgpu.BlendOperationAdd,
		SrcFactor: wgpu.BlendFactorOne,
		DstFactor: wgpu.BlendFactorOneMinusSrcAlpha},
		Alpha: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorOne,
			DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
		},
	}
}

// pick selects the additive variant of a pipeline when the run asks for it.
func pick(additive bool, add, alpha *wgpu.RenderPipeline) *wgpu.RenderPipeline {
	if additive {
		return add
	}
	return alpha
}

// additiveStraightBlend is the additive operator for pipelines whose shaders
// output straight (non-premultiplied) alpha: the source color is scaled by
// its own alpha before being summed, so a faint glow adds faintly.
func additiveStraightBlend() *wgpu.BlendState {
	return &wgpu.BlendState{
		Color: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorSrcAlpha,
			DstFactor: wgpu.BlendFactorOne},
		Alpha: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorOne,
			DstFactor: wgpu.BlendFactorOne},
	}
}

func additiveBlend() *wgpu.BlendState {
	return &wgpu.BlendState{
		Color: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorOne,
			DstFactor: wgpu.BlendFactorOne},
		Alpha: wgpu.BlendComponent{
			Operation: wgpu.BlendOperationAdd,
			SrcFactor: wgpu.BlendFactorOne,
			DstFactor: wgpu.BlendFactorOne},
	}
}

func solidVertexLayout() []wgpu.VertexBufferLayout {
	return []wgpu.VertexBufferLayout{
		{
			ArrayStride: tess.SolidVertexStride,
			StepMode:    wgpu.VertexStepModeVertex,
			Attributes: []wgpu.VertexAttribute{
				{ShaderLocation: 0, Offset: 0, Format: wgpu.VertexFormatFloat32x2},
				{ShaderLocation: 1, Offset: 8, Format: wgpu.VertexFormatFloat32x4},
			},
		},
	}
}

func texturedVertexLayout() []wgpu.VertexBufferLayout {
	return []wgpu.VertexBufferLayout{
		{
			ArrayStride: tess.TexVertexStride,
			StepMode:    wgpu.VertexStepModeVertex,
			Attributes: []wgpu.VertexAttribute{
				{ShaderLocation: 0, Offset: 0, Format: wgpu.VertexFormatFloat32x4},
				{ShaderLocation: 1, Offset: 16, Format: wgpu.VertexFormatFloat32x4},
				{ShaderLocation: 2, Offset: 32, Format: wgpu.VertexFormatFloat32x2},
			},
		},
	}
}

func releaseBuffer(buffer **wgpu.Buffer) {
	if *buffer != nil {
		(*buffer).Release()
		*buffer = nil
	}
}

func releasePipeline(pipeline **wgpu.RenderPipeline) {
	if *pipeline != nil {
		(*pipeline).Release()
		*pipeline = nil
	}
}

func releaseSampler(sampler **wgpu.Sampler) {
	if *sampler != nil {
		(*sampler).Release()
		*sampler = nil
	}
}

func releaseTextureView(view **wgpu.TextureView) {
	if *view != nil {
		(*view).Release()
		*view = nil
	}
}

func releaseTexture(texture **wgpu.Texture) {
	if *texture != nil {
		(*texture).Destroy()
		(*texture).Release()
		*texture = nil
	}
}

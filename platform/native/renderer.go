//go:build windows && amd64

package native

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/internal/diagnostic"
	"github.com/dhannyell/frameforge/internal/render/gpu/fontatlas"
	"github.com/dhannyell/frameforge/internal/render/gpu/shaders"
	"github.com/dhannyell/frameforge/internal/render/gpu/tess"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/go-gl/glfw/v3.4/glfw"
	"github.com/oliverbestmann/webgpu/wgpu"
)

const (
	msaaSampleCount  = 4
	initialBufferCap = 64 * 1024
)

// Renderer is the provisional streaming renderer for a native GLFW window.
// A later host API may wrap this constructor with lifecycle and input options.
type Renderer struct {
	device *device
	clear  render.Color
	log    *slog.Logger

	fontProvider *fontatlas.Provider
	particles    *gpuParticles
	textures     []gpuTexture
	texSizes     tess.TexSizes
	textAtlases  map[tess.AtlasKey]gpuTextAtlas
	tess         tess.Builder

	solidPipeline     *wgpu.RenderPipeline
	texturedPipeline  *wgpu.RenderPipeline
	spritePipeline    *wgpu.RenderPipeline
	textGlyphPipeline *wgpu.RenderPipeline
	mipPipeline       *wgpu.RenderPipeline
	textureSampler    *wgpu.Sampler
	mipSampler        *wgpu.Sampler

	solidBuffer     *wgpu.Buffer
	solidCap        int
	texBuffer       *wgpu.Buffer
	texCap          int
	spriteBuffer    *wgpu.Buffer
	spriteCap       int
	textGlyphBuffer *wgpu.Buffer
	textGlyphCap    int
	textLabelBuffer *wgpu.Buffer
	textLabelCap    int
	spriteUniform   *wgpu.Buffer

	msaaTexture *wgpu.Texture
	msaaView    *wgpu.TextureView
	msaaWidth   int
	msaaHeight  int

	diagnostics        diagnostic.Once
	err                error
	timeCPU            bool
	cpuTess, cpuUpload time.Duration
	cpuEncode          time.Duration
	cpuPhasesSet       bool
}

var _ runtime.FrameRenderer = (*Renderer)(nil)

// NewRenderer creates a native streaming renderer for window using vsync to
// select its surface presentation mode.
func NewRenderer(
	window *glfw.Window,
	clear render.Color,
	manifest asset.Manifest,
	vsync VSyncMode,
	log *slog.Logger,
) (renderer *Renderer, err error) {
	if log == nil {
		log = slog.Default()
	}
	provider, err := fontatlas.New(manifest)
	if err != nil {
		return nil, fmt.Errorf("create font atlas provider: %w", err)
	}
	d, err := newDevice(window, vsync, log)
	if err != nil {
		return nil, err
	}
	renderer = &Renderer{
		device:       d,
		clear:        clear,
		log:          log,
		fontProvider: provider,
		textAtlases:  make(map[tess.AtlasKey]gpuTextAtlas),
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
	renderer.particles, err = newGpuParticles(d.device, d.queue, manifest, renderer.textures, d.config.Format)
	if err != nil {
		// Particle effects are optional presentation. Keep the native host usable
		// when a driver rejects the compute/indirect pipeline, matching the WASM
		// backend's best-effort particle path.
		log.Warn("native particle renderer disabled", "err", err)
		renderer.particles = nil
	}
	return renderer, nil
}

// UpdateResources applies dynamic texture updates. Retained batches are not
// part of the native streaming renderer and are deliberately skipped.
func (r *Renderer) UpdateResources(updates runtime.FrameUpdates) {
	if r == nil || r.err != nil {
		return
	}
	if err := r.textureResources().updateTextures(updates.Textures, r.textures); err != nil {
		r.recordErr(err)
		return
	}
	if len(updates.SpriteBatches) > 0 {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "native-sprite-batches-unsupported"},
			"native renderer skips retained sprite batches")
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
	if r == nil || r.err != nil || r.device == nil || r.device.config == nil {
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
	solid, textured := r.tess.Streams()
	sprites := r.tess.SpriteStream()
	textGlyphs := r.tess.TextGlyphStream()
	textLabels := r.tess.TextLabelStream()
	if r.timeCPU {
		r.cpuTess = time.Since(phaseStart)
		phaseStart = time.Now()
	}
	if err := r.uploadStreams(solid, textured, sprites, textGlyphs, textLabels, frame.Camera, width, height); err != nil {
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
	surfaceTexture, err := r.device.surface.TryGetCurrentTexture()
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
	if err := r.encodeFrame(runs, surfaceView, frame.Camera, width, height); err != nil {
		r.recordErr(err)
		return
	}
	if r.timeCPU {
		r.cpuEncode = time.Since(phaseStart)
		r.cpuPhasesSet = true
	}
	r.device.surface.Present()
	if r.particles != nil {
		r.particles.endFrame()
	}
}

// SetCPUTiming enables CPU-side phase instrumentation.
func (r *Renderer) SetCPUTiming(enabled bool) {
	if r != nil {
		r.timeCPU = enabled
	}
}

// SetGPUTiming is retained for option compatibility. The current binding's
// C-to-Go completion callback is unsafe, so native GPU timing stays unavailable.
func (r *Renderer) SetGPUTiming(bool) {}

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
	for _, texture := range r.textures {
		texture.release()
	}
	r.textures = nil
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
	releasePipeline(&r.mipPipeline)
	releaseSampler(&r.textureSampler)
	releaseSampler(&r.mipSampler)
	if r.device != nil {
		r.device.destroy()
		r.device = nil
	}
}

func (r *Renderer) createPipelines() error {
	var err error
	if r.solidPipeline, err = r.createPipeline(
		"solid",
		shaders.SolidWGSL,
		r.device.config.Format,
		msaaSampleCount,
		solidVertexLayout(),
		alphaBlend(),
	); err != nil {
		return err
	}

	if r.texturedPipeline, err = r.createPipeline(
		"textured",
		shaders.TexturedWGSL,
		r.device.config.Format,
		msaaSampleCount,
		texturedVertexLayout(),
		premultipliedBlend(),
	); err != nil {
		return err
	}

	if r.spritePipeline, err = r.createPipeline(
		"sprite",
		shaders.SpriteWGSL,
		r.device.config.Format,
		msaaSampleCount,
		nil,
		premultipliedBlend(),
	); err != nil {
		return err
	}

	if r.textGlyphPipeline, err = r.createPipeline(
		"text glyph",
		shaders.TextGlyphWGSL,
		r.device.config.Format,
		msaaSampleCount,
		nil,
		alphaBlend(),
	); err != nil {
		return err
	}

	r.mipPipeline, err = r.createPipeline("mipmap", shaders.MipmapWGSL, wgpu.TextureFormatRGBA8Unorm, 1, nil, nil)
	return err
}

func (r *Renderer) createPipeline(
	label, source string,
	format wgpu.TextureFormat,
	samples uint32,
	buffers []wgpu.VertexBufferLayout,
	blend *wgpu.BlendState,
) (*wgpu.RenderPipeline, error) {
	module, err := r.device.device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "frameforge " + label + " shader",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: source},
	})
	if err != nil {
		return nil, fmt.Errorf("create %s shader: %w", label, err)
	}
	defer module.Release()
	pipeline, err := r.device.device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:       "frameforge " + label + " pipeline",
		Layout:      nil,
		Vertex:      wgpu.VertexState{Module: module, EntryPoint: "vs_main", Buffers: buffers},
		Primitive:   wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList},
		Multisample: wgpu.MultisampleState{Count: samples, Mask: ^uint32(0)},
		Fragment: &wgpu.FragmentState{Module: module, EntryPoint: "fs_main", Targets: []wgpu.ColorTargetState{{
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
		initialBufferCap,
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.solidCap = initialBufferCap

	if r.texBuffer, err = r.newBuffer(
		"textured vertices",
		initialBufferCap,
		wgpu.BufferUsageVertex|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.texCap = initialBufferCap

	if r.spriteBuffer, err = r.newBuffer(
		"sprite records",
		initialBufferCap,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.spriteCap = initialBufferCap

	if r.textGlyphBuffer, err = r.newBuffer(
		"text glyph records",
		initialBufferCap,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.textGlyphCap = initialBufferCap

	if r.textLabelBuffer, err = r.newBuffer(
		"text label records",
		initialBufferCap,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
	); err != nil {
		return err
	}
	r.textLabelCap = initialBufferCap
	r.spriteUniform, err = r.newBuffer(
		"sprite uniform",
		tess.SpriteUniformSize,
		wgpu.BufferUsageUniform|wgpu.BufferUsageCopyDst,
	)
	return err
}

func (r *Renderer) newBuffer(label string, size int, usage wgpu.BufferUsage) (*wgpu.Buffer, error) {
	buffer, err := r.device.device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Label: "frameforge " + label,
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
	camera render.Camera,
	width, height int,
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
	if len(sprites) > 0 || len(glyphs) > 0 {
		var uniforms [tess.SpriteUniformSize]byte
		tess.PackSpriteUniforms(uniforms[:], camera, width, height)
		if err := r.device.queue.TryWriteBuffer(r.spriteUniform, 0, uniforms[:]); err != nil {
			return fmt.Errorf("upload sprite uniform: %w", err)
		}
	}
	return nil
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
	next := growBufferCap(current, required)
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
			Label:  "frameforge sprite bind group",
			Layout: layout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, TextureView: texture.view},
				{Binding: 1, Sampler: r.textureSampler},
				{Binding: 2, Buffer: r.spriteBuffer, Size: wgpu.WholeSize},
				{Binding: 3, Buffer: r.spriteUniform, Size: wgpu.WholeSize},
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
	if r.msaaTexture != nil && r.msaaWidth == width && r.msaaHeight == height {
		return nil
	}
	releaseTextureView(&r.msaaView)
	releaseTexture(&r.msaaTexture)
	texture, err := r.device.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "frameforge MSAA target",
		Usage:         wgpu.TextureUsageRenderAttachment,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		Format:        r.device.config.Format,
		MipLevelCount: 1,
		SampleCount:   msaaSampleCount,
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
	surfaceView *wgpu.TextureView,
	camera render.Camera,
	width, height int,
) error {
	encoder, err := r.device.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("create render command encoder: %w", err)
	}
	defer encoder.Release()
	if r.particles != nil {
		sims := make([]particleSim, 0)
		for _, run := range runs {
			if run.Kind != tess.DrawRunParticles {
				continue
			}
			if sim, ok := r.particles.prepare(run.Particles, camera, width, height); ok {
				sims = append(sims, sim)
			}
		}
		if err := r.particles.encode(encoder, sims); err != nil {
			return fmt.Errorf("encode particle simulation: %w", err)
		}
	}
	pass, err := encoder.TryBeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "frameforge render pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:          r.msaaView,
				ResolveTarget: surfaceView,
				LoadOp:        wgpu.LoadOpClear,
				StoreOp:       wgpu.StoreOpDiscard,
				ClearValue:    clearColor(r.clear),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("begin render pass: %w", err)
	}
	defer pass.Release()
	for _, run := range runs {
		if err := r.drawRun(pass, run, camera, width, height); err != nil {
			return err
		}
	}
	if err := pass.TryEnd(); err != nil {
		return fmt.Errorf("end render pass: %w", err)
	}
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish render command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.device.queue.Submit(commandBuffer)
	return nil
}

func (r *Renderer) drawRun(pass *wgpu.RenderPassEncoder, run tess.StreamRun, camera render.Camera, width, height int) error {
	switch run.Kind {
	case tess.DrawRunSolid:
		pass.SetPipeline(r.solidPipeline)
		pass.SetVertexBuffer(0, r.solidBuffer, 0, uint64(r.solidCap))
		pass.Draw(uint32(run.VertexCount), 1, uint32(run.ByteOffset/tess.SolidVertexStride), 0)
	case tess.DrawRunTextured:
		texture, ok := textureFor(r.textures, run.Texture)
		if !ok {
			return nil
		}
		pass.SetPipeline(r.texturedPipeline)
		pass.SetVertexBuffer(0, r.texBuffer, 0, uint64(r.texCap))
		pass.SetBindGroup(0, texture.bindGroup, nil)
		pass.Draw(uint32(run.VertexCount), 1, uint32(run.ByteOffset/tess.TexVertexStride), 0)
	case tess.DrawRunSprites:
		texture, ok := textureFor(r.textures, run.Texture)
		if !ok || texture.spriteBindGroup == nil {
			return nil
		}
		pass.SetPipeline(r.spritePipeline)
		pass.SetBindGroup(0, texture.spriteBindGroup, nil)
		pass.Draw(uint32(run.VertexCount*6), 1, uint32(run.ByteOffset/tess.SpriteStride*6), 0)
	case tess.DrawRunText:
		atlas, ok, err := r.textAtlasResources().textAtlasFor(r.textAtlases, r.fontProvider, run.AtlasKey)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		pass.SetPipeline(r.textGlyphPipeline)
		pass.SetBindGroup(0, atlas.glyphBindGroup, nil)
		pass.Draw(uint32(run.VertexCount*6), 1, uint32(run.ByteOffset/tess.TextGlyphStride*6), 0)
	case tess.DrawRunParticles:
		if r.particles != nil {
			r.particles.draw(pass, run.Particles, camera, width, height)
		}
	case tess.DrawRunSpriteBatch:
		r.warnSkipped("sprite batches")
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
func growBufferCap(current, required int) int {
	if current <= 0 {
		current = initialBufferCap
	}
	for current < required {
		current *= 2
	}
	return current
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

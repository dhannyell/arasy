// Package gpu defines the testable resource-setup seam of the WebGPU backend.
package gpu

// Handle is an opaque reference to a GPU-side object. The js adapter wraps a
// js.Value; Fake hands out sequential handles. Pure code never looks inside.
type Handle any

// Device creates GPU resources and writes their initial contents. Per-frame
// command encoding intentionally stays on the js side of the WebGPU backend.
type Device interface {
	CreateShaderModule(code string) Handle
	CreateRenderPipeline(desc map[string]any) Handle
	CreateSampler(desc map[string]any) Handle
	CreateTexture(desc map[string]any) Handle
	CreateBuffer(desc map[string]any) Handle
	CreateBindGroup(desc map[string]any) Handle
	CreateView(texture Handle, desc map[string]any) Handle
	BindGroupLayout(pipeline Handle, index int) Handle
	WriteBuffer(buffer Handle, offset int, data []byte)
	WriteTexture(dst map[string]any, data []byte, layout, size map[string]any)
}

// WebGPU usage flags from WebGPU 5.1.1, GPUTextureUsage and GPUBufferUsage.
// Their numeric values are normative and stable in the WebGPU specification.
const (
	TextureUsageCopySrc          = 0x01
	TextureUsageCopyDst          = 0x02
	TextureUsageTextureBinding   = 0x04
	TextureUsageStorageBinding   = 0x08
	TextureUsageRenderAttachment = 0x10

	BufferUsageMapRead      = 0x0001
	BufferUsageMapWrite     = 0x0002
	BufferUsageCopySrc      = 0x0004
	BufferUsageCopyDst      = 0x0008
	BufferUsageIndex        = 0x0010
	BufferUsageVertex       = 0x0020
	BufferUsageUniform      = 0x0040
	BufferUsageStorage      = 0x0080
	BufferUsageIndirect     = 0x0100
	BufferUsageQueryResolve = 0x0200
)

package gpu_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/platform/wasm/gpu"
)

func TestPipelineDescriptorsMatchLiteralFixtures(t *testing.T) {
	t.Parallel()
	module := gpu.Handle("module")
	format := "bgra8unorm"

	tests := []struct {
		name string
		got  map[string]any
		want map[string]any
	}{
		{"solid", gpu.SolidPipelineDesc(module, format, 4, gpu.AlphaBlend()), literalPipeline(module, format, 4, 24, []any{
			map[string]any{"shaderLocation": 0, "offset": 0, "format": "float32x2"},
			map[string]any{"shaderLocation": 1, "offset": 8, "format": "float32x4"},
		}, literalAlphaBlend())},
		{"textured", gpu.TexturedPipelineDesc(module, format, 4, gpu.PremultipliedBlend()), literalPipeline(module, format, 4, 40, []any{
			map[string]any{"shaderLocation": 0, "offset": 0, "format": "float32x4"},
			map[string]any{"shaderLocation": 1, "offset": 16, "format": "float32x4"},
			map[string]any{"shaderLocation": 2, "offset": 32, "format": "float32x2"},
		}, literalPremultipliedBlend())},
		{"sprites", gpu.SpritePipelineDesc(module, format, 4, gpu.PremultipliedBlend()), map[string]any{
			"layout":      "auto",
			"vertex":      map[string]any{"module": module, "entryPoint": "vs_main"},
			"fragment":    map[string]any{"module": module, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": format, "blend": literalPremultipliedBlend()}}},
			"primitive":   map[string]any{"topology": "triangle-list"},
			"multisample": map[string]any{"count": 4},
		}},
		{"text glyphs", gpu.TextGlyphPipelineDesc(module, format, 4, gpu.AlphaBlend()), map[string]any{
			"layout":      "auto",
			"vertex":      map[string]any{"module": module, "entryPoint": "vs_main"},
			"fragment":    map[string]any{"module": module, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": format, "blend": literalAlphaBlend()}}},
			"primitive":   map[string]any{"topology": "triangle-list"},
			"multisample": map[string]any{"count": 4},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("descriptor differs (-want +got):\nwant: %#v\n got: %#v", tt.want, tt.got)
			}
		})
	}
}

func TestBlendDescriptorsRemainDistinct(t *testing.T) {
	t.Parallel()
	alpha := gpu.AlphaBlend()
	premultiplied := gpu.PremultipliedBlend()
	additive := gpu.AdditiveBlend()
	if reflect.DeepEqual(alpha, premultiplied) || reflect.DeepEqual(alpha, additive) || reflect.DeepEqual(premultiplied, additive) {
		t.Fatal("blend descriptors must remain distinct")
	}
	if alpha["color"].(map[string]any)["srcFactor"] != "src-alpha" ||
		premultiplied["color"].(map[string]any)["srcFactor"] != "one" ||
		additive["color"].(map[string]any)["dstFactor"] != "one" {
		t.Fatal("blend factors changed")
	}
}

func TestTextureAndParticleBufferDescriptors(t *testing.T) {
	t.Parallel()
	texture := gpu.TextureDesc(8, 8, 4)
	if texture["usage"] != gpu.TextureUsageTextureBinding|gpu.TextureUsageCopyDst|gpu.TextureUsageRenderAttachment {
		t.Fatalf("texture usage = %v", texture["usage"])
	}
	if got, want := gpu.TextureArrayDesc(8, 4, 3, 4), map[string]any{
		"size":   map[string]any{"width": 8, "height": 4, "depthOrArrayLayers": 3},
		"format": "rgba8unorm", "mipLevelCount": 4,
		"usage": gpu.TextureUsageTextureBinding | gpu.TextureUsageCopyDst | gpu.TextureUsageRenderAttachment,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("texture-array descriptor differs: got %#v, want %#v", got, want)
	}
	if got, want := gpu.VertexBufferDesc(10)["usage"], gpu.BufferUsageVertex|gpu.BufferUsageCopyDst; got != want {
		t.Fatalf("vertex usage = %v, want %v", got, want)
	}
	particle := gpu.BufferDesc(100*gpu.ParticleStride, gpu.BufferUsageStorage|gpu.BufferUsageCopyDst)
	if got, want := particle["size"], 4800; got != want {
		t.Fatalf("particle buffer bytes = %v, want %v", got, want)
	}
	if got, want := particle["usage"], gpu.BufferUsageStorage|gpu.BufferUsageCopyDst; got != want {
		t.Fatalf("particle buffer usage = %v, want %v", got, want)
	}
	if gpu.ParticleUniformSize != 224 {
		t.Fatalf("particle uniform size = %d, want 224", gpu.ParticleUniformSize)
	}
}

func TestOtherDescriptorsMatchLiteralFixtures(t *testing.T) {
	t.Parallel()
	module := gpu.Handle("mip-module")
	if got, want := gpu.MipPipelineDesc(module), map[string]any{
		"layout":    "auto",
		"vertex":    map[string]any{"module": module, "entryPoint": "vs_main"},
		"fragment":  map[string]any{"module": module, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": "rgba8unorm"}}},
		"primitive": map[string]any{"topology": "triangle-list"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mip descriptor differs: got %#v, want %#v", got, want)
	}
	if got, want := gpu.TextureSamplerDesc(), map[string]any{"magFilter": "linear", "minFilter": "linear", "mipmapFilter": "linear", "maxAnisotropy": 16}; !reflect.DeepEqual(got, want) {
		t.Fatalf("texture sampler differs: got %#v, want %#v", got, want)
	}
	if got, want := gpu.MipSamplerDesc(), map[string]any{"magFilter": "linear", "minFilter": "linear"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mip sampler differs: got %#v, want %#v", got, want)
	}
	if got, want := gpu.NearestSamplerDesc(), map[string]any{"magFilter": "nearest", "minFilter": "nearest"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nearest sampler differs: got %#v, want %#v", got, want)
	}
	if got, want := gpu.LinearSamplerDesc(), map[string]any{"magFilter": "linear", "minFilter": "linear"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("linear sampler differs: got %#v, want %#v", got, want)
	}
	if got, want := gpu.R8TextureDesc(2, 3), map[string]any{"size": []any{2, 3}, "format": "r8unorm", "usage": gpu.TextureUsageTextureBinding | gpu.TextureUsageCopyDst}; !reflect.DeepEqual(got, want) {
		t.Fatalf("r8 texture differs: got %#v, want %#v", got, want)
	}
}

func literalPipeline(module gpu.Handle, format string, sampleCount, stride int, attributes []any, blend map[string]any) map[string]any {
	return map[string]any{
		"layout":    "auto",
		"vertex":    map[string]any{"module": module, "entryPoint": "vs_main", "buffers": []any{map[string]any{"arrayStride": stride, "attributes": attributes}}},
		"fragment":  map[string]any{"module": module, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": format, "blend": blend}}},
		"primitive": map[string]any{"topology": "triangle-list"}, "multisample": map[string]any{"count": sampleCount},
	}
}

func literalAlphaBlend() map[string]any {
	return map[string]any{"color": map[string]any{"srcFactor": "src-alpha", "dstFactor": "one-minus-src-alpha", "operation": "add"}, "alpha": map[string]any{"srcFactor": "one", "dstFactor": "one-minus-src-alpha", "operation": "add"}}
}

func literalPremultipliedBlend() map[string]any {
	return map[string]any{"color": map[string]any{"srcFactor": "one", "dstFactor": "one-minus-src-alpha", "operation": "add"}, "alpha": map[string]any{"srcFactor": "one", "dstFactor": "one-minus-src-alpha", "operation": "add"}}
}

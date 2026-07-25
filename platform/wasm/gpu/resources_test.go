package gpu_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/platform/wasm/gpu"
)

func TestBuildTextureResourcesRecordsSetupInOrder(t *testing.T) {
	t.Parallel()
	fake := &gpu.Fake{}
	resources := gpu.BuildTextureResources(fake, make([]byte, 8*8*4), 8, 8, "pipeline", "sampler")
	if resources.Texture == nil || resources.View == nil || resources.BindGroup == nil {
		t.Fatal("texture resources contain a nil handle")
	}
	gotMethods := make([]string, len(fake.Calls))
	for i, call := range fake.Calls {
		gotMethods[i] = call.Method
	}
	wantMethods := []string{"CreateTexture", "WriteTexture", "CreateView", "BindGroupLayout", "CreateBindGroup"}
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Fatalf("call order = %v, want %v", gotMethods, wantMethods)
	}
	texture := fake.Calls[0].Args[0].(map[string]any)
	if texture["mipLevelCount"] != 4 {
		t.Fatalf("mipLevelCount = %v, want 4", texture["mipLevelCount"])
	}
	if texture["usage"] != gpu.TextureUsageTextureBinding|gpu.TextureUsageCopyDst|gpu.TextureUsageRenderAttachment {
		t.Fatalf("usage = %v", texture["usage"])
	}
	layout := fake.Calls[1].Args[2].(map[string]any)
	if layout["bytesPerRow"] != 32 || layout["rowsPerImage"] != 8 {
		t.Fatalf("write layout = %#v", layout)
	}
}

func TestMipLevelCount(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ w, h, want int }{{1, 1, 1}, {8, 8, 4}, {3, 7, 3}} {
		if got := gpu.MipLevelCount(tc.w, tc.h); got != tc.want {
			t.Errorf("MipLevelCount(%d, %d) = %d, want %d", tc.w, tc.h, got, tc.want)
		}
	}
}

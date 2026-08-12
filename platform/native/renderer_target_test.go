//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/oliverbestmann/webgpu/wgpu"
)

func TestResolveFramePassesSkipsUnavailableTargetOnce(t *testing.T) {
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"dynamic":[{"name":"target","w":4,"h":4}]}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	renderer := Renderer{
		manifest: manifest,
		log:      slog.New(slog.NewTextHandler(&logs, nil)),
	}
	frame := runtime.Frame{Passes: []render.Pass{{Target: manifest.Texture("target")}}}

	for range 2 {
		passes := renderer.resolveFramePasses(frame, 8, 8)
		if len(passes) != 1 || passes[0].Target != 0 {
			t.Fatalf("resolved passes = %#v, want base pass only", passes)
		}
	}
	if got := strings.Count(logs.String(), "pass target activation failed"); got != 1 {
		t.Fatalf("unavailable-target warnings = %d, want 1; logs: %s", got, logs.String())
	}
}

func TestNativeTextureSampleViewUsesMipZeroAfterRTT(t *testing.T) {
	full := &wgpu.TextureView{}
	target := &wgpu.TextureView{}
	texture := gpuTexture{view: full, targetView: target}
	if got := nativeTextureSampleView(texture); got != full {
		t.Fatalf("static sample view = %p, want full view %p", got, full)
	}
	texture.rtt = true
	if got := nativeTextureSampleView(texture); got != target {
		t.Fatalf("RTT sample view = %p, want mip-zero view %p", got, target)
	}
}

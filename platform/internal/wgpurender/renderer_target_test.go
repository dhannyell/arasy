//go:build (windows || linux || darwin) && (amd64 || arm64)

package wgpurender

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
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
	texture := gpuTexture{view: full, targetView: target, arrayLayer: -1}
	if got := nativeTextureSampleView(texture); got != full {
		t.Fatalf("static sample view = %p, want full view %p", got, full)
	}
	texture.rtt = true
	if got := nativeTextureSampleView(texture); got != target {
		t.Fatalf("RTT sample view = %p, want mip-zero view %p", got, target)
	}
}

func TestCPUFrameTimingUsesSubmitBoundaryAndExcludesAcquireWait(t *testing.T) {
	base := time.Unix(100, 0)
	tickEnd := base.Add(2 * time.Millisecond)
	inputAt := base.Add(-3 * time.Millisecond)
	timing := CPUFrameTiming{
		submittedAt: base.Add(8 * time.Millisecond),
		cpuEndAt:    base.Add(10 * time.Millisecond),
		acquireWait: 4 * time.Millisecond,
		Valid:       true,
	}

	if got, want := timing.RenderDurationSince(tickEnd), 4*time.Millisecond; got != want {
		t.Fatalf("CPU render duration = %v, want %v", got, want)
	}
	latency, ok := timing.InputToSubmit(inputAt)
	if !ok {
		t.Fatal("input-to-submit timing unavailable")
	}
	if want := 11 * time.Millisecond; latency != want {
		t.Fatalf("input-to-submit = %v, want %v", latency, want)
	}
}

func TestDecideSurfaceStatus(t *testing.T) {
	tests := []struct {
		name   string
		status wgpu.SurfaceGetCurrentTextureStatus
		want   surfaceAction
	}{
		{name: "optimal", status: wgpu.SurfaceGetCurrentTextureStatusSuccessOptimal, want: actionRender},
		{name: "suboptimal", status: wgpu.SurfaceGetCurrentTextureStatusSuccessSuboptimal, want: actionRender},
		{name: "timeout", status: wgpu.SurfaceGetCurrentTextureStatusTimeout, want: actionSkip},
		{name: "occluded", status: wgpu.SurfaceGetCurrentTextureStatusOccluded, want: actionSkip},
		{name: "outdated", status: wgpu.SurfaceGetCurrentTextureStatusOutdated, want: actionReconfigure},
		{name: "lost", status: wgpu.SurfaceGetCurrentTextureStatusLost, want: actionReconfigure},
		{name: "error", status: wgpu.SurfaceGetCurrentTextureStatusError, want: actionFatal},
		{name: "unknown", status: wgpu.SurfaceGetCurrentTextureStatus(99), want: actionFatal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decideSurfaceStatus(test.status); got != test.want {
				t.Fatalf("decideSurfaceStatus(%v) = %v, want %v", test.status, got, test.want)
			}
		})
	}
}

func TestMipLevelCount(t *testing.T) {
	for _, tc := range []struct{ w, h, want int }{{1, 1, 1}, {8, 8, 4}, {3, 7, 3}} {
		if got := mipLevelCount(tc.w, tc.h); got != tc.want {
			t.Errorf("mipLevelCount(%d, %d) = %d, want %d", tc.w, tc.h, got, tc.want)
		}
	}
}

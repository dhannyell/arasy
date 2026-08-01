//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"io"
	"log/slog"
	"testing"

	"github.com/oliverbestmann/webgpu/wgpu"
)

func TestChooseSurfaceFormat(t *testing.T) {
	tests := []struct {
		name    string
		formats []wgpu.TextureFormat
		want    wgpu.TextureFormat
		wantErr bool
	}{
		{name: "prefers BGRA", formats: []wgpu.TextureFormat{wgpu.TextureFormatRGBA8Unorm, wgpu.TextureFormatBGRA8Unorm}, want: wgpu.TextureFormatBGRA8Unorm},
		{name: "falls back to RGBA", formats: []wgpu.TextureFormat{wgpu.TextureFormatRGBA8Unorm}, want: wgpu.TextureFormatRGBA8Unorm},
		{name: "rejects sRGB only", formats: []wgpu.TextureFormat{wgpu.TextureFormatBGRA8UnormSrgb}, wantErr: true},
		{name: "rejects empty", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := chooseSurfaceFormat(test.formats)
			if test.wantErr {
				if err == nil {
					t.Fatal("chooseSurfaceFormat() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("chooseSurfaceFormat() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestChooseAlphaMode(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		modes   []wgpu.CompositeAlphaMode
		want    wgpu.CompositeAlphaMode
		wantErr bool
	}{
		{name: "prefers opaque", modes: []wgpu.CompositeAlphaMode{wgpu.CompositeAlphaModePremultiplied, wgpu.CompositeAlphaModeOpaque}, want: wgpu.CompositeAlphaModeOpaque},
		{name: "falls back to first supported mode", modes: []wgpu.CompositeAlphaMode{wgpu.CompositeAlphaModePremultiplied}, want: wgpu.CompositeAlphaModePremultiplied},
		{name: "rejects empty", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := chooseAlphaMode(test.modes, log)
			if test.wantErr {
				if err == nil {
					t.Fatal("chooseAlphaMode() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("chooseAlphaMode() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestChoosePresentMode(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		modes   []wgpu.PresentMode
		vsync   VSyncMode
		want    wgpu.PresentMode
		wantErr bool
	}{
		{name: "rejects missing fifo", modes: []wgpu.PresentMode{wgpu.PresentModeMailbox}, wantErr: true},
		{name: "vsync on selects fifo", modes: []wgpu.PresentMode{wgpu.PresentModeMailbox, wgpu.PresentModeFifo}, vsync: VSyncOn, want: wgpu.PresentModeFifo},
		{name: "vsync off prefers immediate", modes: []wgpu.PresentMode{wgpu.PresentModeFifo, wgpu.PresentModeImmediate, wgpu.PresentModeMailbox}, vsync: VSyncOff, want: wgpu.PresentModeImmediate},
		{name: "vsync off falls back to mailbox", modes: []wgpu.PresentMode{wgpu.PresentModeFifo, wgpu.PresentModeMailbox}, vsync: VSyncOff, want: wgpu.PresentModeMailbox},
		{name: "vsync off falls back to fifo", modes: []wgpu.PresentMode{wgpu.PresentModeFifo}, vsync: VSyncOff, want: wgpu.PresentModeFifo},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := choosePresentMode(test.modes, test.vsync, log)
			if test.wantErr {
				if err == nil {
					t.Fatal("choosePresentMode() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("choosePresentMode() = %v, want %v", got, test.want)
			}
		})
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

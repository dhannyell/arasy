//go:build windows && amd64

package sysprobe

import (
	"testing"
	"time"
)

func TestProberSmoke(t *testing.T) {
	p := NewProber(0, 0)
	defer p.Close()
	s := p.Sample(time.Now())

	if !s.MemOK || s.MemProcessBytes == 0 || s.MemSystemTotalBytes == 0 {
		t.Fatalf("memory probe failed: %+v", s)
	}
	t.Logf("mem: process=%dMB sys=%d/%dMB",
		s.MemProcessBytes>>20, s.MemSystemUsedBytes>>20, s.MemSystemTotalBytes>>20)
	t.Logf("vram: ok=%v process=%dMB budget=%dMB",
		s.VRAMOK, s.VRAMProcessBytes>>20, s.VRAMBudgetBytes>>20)
	t.Logf("gpu: ok=%v process=%.1f%% system=%.1f%%",
		s.GPUOK, s.GPUProcessPercent, s.GPUSystemPercent)
}

// The VRAM walk keeps its DXGI factory across samples, so a second sample
// must read through the cached one rather than build another.
func TestVideoMemoryReusesTheFactory(t *testing.T) {
	p := NewProber(0, 0)
	defer p.Close()

	if _, _, ok := p.videoMemory(); !ok {
		t.Skip("no DXGI adapter with dedicated video memory on this machine")
	}
	cached := p.factory
	if cached == nil {
		t.Fatal("a successful walk left no factory cached")
	}
	if _, _, ok := p.videoMemory(); !ok {
		t.Fatal("the second walk failed through the cached factory")
	}
	if p.factory != cached {
		t.Fatal("the second walk rebuilt the factory")
	}
}

// Close must be safe to call on a prober whose probes never attached, and
// safe to call twice.
func TestCloseIsIdempotent(t *testing.T) {
	p := NewProber(0, 0)
	p.Close()
	p.Close()
}

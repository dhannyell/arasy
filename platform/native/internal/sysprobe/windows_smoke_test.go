//go:build windows && amd64

package sysprobe

import (
	"testing"
	"time"
)

func TestProberSmoke(t *testing.T) {
	p := NewProber(0, 0)
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

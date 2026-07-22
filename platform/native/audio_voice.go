//go:build windows && amd64

package native

import (
	"encoding/binary"
	"io"
	"math"
	"sync"
)

// Voice converts decoded interleaved stereo PCM into the format consumed by oto.
// Gain ramps are advanced once per output frame, so their duration is independent
// of the size of the buffers requested by the audio device.
type voice struct {
	mu sync.Mutex

	pcm  []int16
	pos  int // Frame offset in pcm.
	loop bool
	pan  float64

	gain          float64
	targetGain    float64
	fadeStep      float64
	fadeRemaining int
	stopAtZero    bool

	pending    [4]byte
	pendingOff int
	pendingLen int
}

func newVoice(pcm []int16, loop bool, pan, gain float64) *voice {
	return &voice{
		pcm:        pcm,
		loop:       loop,
		pan:        clampPan(pan),
		gain:       gain,
		targetGain: gain,
	}
}

// Read produces little-endian, interleaved signed 16-bit stereo PCM.
func (v *voice) Read(p []byte) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	n := 0
	for n < len(p) {
		if v.pendingOff == v.pendingLen {
			if !v.nextFrame() {
				if n == 0 {
					return 0, io.EOF
				}
				return n, io.EOF
			}
		}
		copied := copy(p[n:], v.pending[v.pendingOff:v.pendingLen])
		n += copied
		v.pendingOff += copied
	}
	return n, nil
}

func (v *voice) nextFrame() bool {
	frames := len(v.pcm) / 2
	if frames == 0 || v.stopAtZero {
		return false
	}
	if v.pos >= frames {
		if !v.loop {
			return false
		}
		v.pos = 0
	}

	left := v.pcm[v.pos*2]
	right := v.pcm[v.pos*2+1]
	v.pos++

	theta := (v.pan + 1) * math.Pi / 4
	leftGain := v.gain * math.Cos(theta)
	rightGain := v.gain * math.Sin(theta)
	binary.LittleEndian.PutUint16(v.pending[:2], uint16(scaleSample(left, leftGain)))
	binary.LittleEndian.PutUint16(v.pending[2:], uint16(scaleSample(right, rightGain)))
	v.pendingOff = 0
	v.pendingLen = len(v.pending)
	v.advanceFade()
	return true
}

func (v *voice) setGain(target, fadeSec float64, rate int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.setGainLocked(target, fadeSec, rate)
}

func (v *voice) setGainLocked(target, fadeSec float64, rate int) {
	fadeSamples := int(fadeSec * float64(rate))
	v.targetGain = target
	v.stopAtZero = false
	if fadeSamples <= 0 {
		v.gain = target
		v.fadeStep = 0
		v.fadeRemaining = 0
		return
	}
	v.fadeStep = (target - v.gain) / float64(fadeSamples)
	v.fadeRemaining = fadeSamples
}

func (v *voice) setPosition(frame int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	frames := len(v.pcm) / 2
	if frames == 0 {
		v.pos = 0
		return
	}
	if v.loop {
		frame %= frames
	}
	v.pos = max(0, min(frame, frames))
	v.pendingOff = 0
	v.pendingLen = 0
}

func (v *voice) fadeOut(fadeSec float64, rate int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.setGainLocked(0, fadeSec, rate)
	if v.fadeRemaining == 0 {
		v.stopAtZero = true
	}
}

func (v *voice) advanceFade() {
	if v.fadeRemaining == 0 {
		return
	}
	v.gain += v.fadeStep
	v.fadeRemaining--
	if v.fadeRemaining == 0 {
		v.gain = v.targetGain
		if v.targetGain == 0 {
			v.stopAtZero = true
		}
	}
}

func clampPan(pan float64) float64 {
	return max(-1, min(1, pan))
}

func scaleSample(sample int16, gain float64) int16 {
	value := float64(sample) * gain
	if value > math.MaxInt16 {
		return math.MaxInt16
	}
	if value < math.MinInt16 {
		return math.MinInt16
	}
	return int16(value)
}

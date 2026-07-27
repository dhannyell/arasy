//go:build windows && amd64

package native

import (
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestVoiceAppliesGain(t *testing.T) {
	v := newVoice([]int16{2000, -4000}, false, -1, 0.5)
	got, err := readVoiceFrames(v, 1)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	want := []int16{1000, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples = %v, want %v", got, want)
	}
}

func TestVoiceFadeIsSampleAccurate(t *testing.T) {
	v := newVoice([]int16{12000, 12000, 12000, 12000, 12000, 12000, 12000, 12000}, false, -1, 0)
	v.setGain(1, 0.003, 1000)

	got, err := readVoiceFrames(v, 4)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	want := []int16{0, 0, 4000, 0, 8000, 0, 12000, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples = %v, want %v", got, want)
	}
}

func TestVoiceFadeOutEndsAtEOF(t *testing.T) {
	v := newVoice([]int16{1000, 1000, 1000, 1000, 1000, 1000}, false, -1, 1)
	v.fadeOut(0.002, 1000)

	got, err := readVoiceFrames(v, 3)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Read() error = %v, want io.EOF", err)
	}
	want := []int16{1000, 0, 500, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples = %v, want %v", got, want)
	}
}

func TestVoicePansWithConstantPower(t *testing.T) {
	tests := []struct {
		name  string
		pan   float64
		check func(t *testing.T, left, right int16)
	}{
		{
			name: "center balances channels",
			pan:  0,
			check: func(t *testing.T, left, right int16) {
				t.Helper()
				if left != right || left < 7000 || left > 7100 {
					t.Errorf("center pan = (%d, %d), want equal constant-power channels", left, right)
				}
			},
		},
		{
			name: "left mutes right",
			pan:  -1,
			check: func(t *testing.T, left, right int16) {
				t.Helper()
				if left != 10000 || right != 0 {
					t.Errorf("left pan = (%d, %d), want (10000, 0)", left, right)
				}
			},
		},
		{
			name: "right mutes left",
			pan:  1,
			check: func(t *testing.T, left, right int16) {
				t.Helper()
				if left != 0 || right != 10000 {
					t.Errorf("right pan = (%d, %d), want (0, 10000)", left, right)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := newVoice([]int16{10000, 10000}, false, test.pan, 1)
			got, err := readVoiceFrames(v, 1)
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			test.check(t, got[0], got[1])
		})
	}
}

func TestVoiceLoops(t *testing.T) {
	v := newVoice([]int16{100, 200, 300, 400}, true, -1, 1)
	got, err := readVoiceFrames(v, 3)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	want := []int16{100, 0, 300, 0, 100, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples = %v, want %v", got, want)
	}
}

func TestVoiceOneShotEndsAtEOF(t *testing.T) {
	v := newVoice([]int16{100, 200}, false, -1, 1)
	got, err := readVoiceFrames(v, 2)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Read() error = %v, want io.EOF", err)
	}
	want := []int16{100, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples = %v, want %v", got, want)
	}
}

func TestVoiceSeekUsesPCMByteOffsets(t *testing.T) {
	v := newVoice([]int16{100, 200, 300, 400}, false, -1, 1)
	offset, err := v.Seek(bytesPerAudioFrame, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	if offset != bytesPerAudioFrame {
		t.Fatalf("Seek() offset = %d, want %d", offset, bytesPerAudioFrame)
	}

	got, err := readVoiceFrames(v, 1)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	want := []int16{300, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples after Seek = %v, want %v", got, want)
	}
}

func readVoiceFrames(v *voice, frames int) ([]int16, error) {
	data := make([]byte, frames*4)
	n, err := v.Read(data)
	if n%2 != 0 {
		return nil, errors.New("voice returned an odd number of PCM bytes")
	}
	samples := make([]int16, n/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return samples, err
}

//go:build windows && amd64

package native

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestDecodeWAVConvertsPCMToStereo(t *testing.T) {
	tests := []struct {
		name     string
		channels uint16
		bits     uint16
		pcm      []byte
		want     []int16
	}{
		{
			name:     "16 bit stereo preserves channels",
			channels: 2,
			bits:     16,
			pcm:      pcm16(-1000, 2000, 3000, -4000),
			want:     []int16{-1000, 2000, 3000, -4000},
		},
		{
			name:     "16 bit mono duplicates each frame",
			channels: 1,
			bits:     16,
			pcm:      pcm16(-1000, 2000),
			want:     []int16{-1000, -1000, 2000, 2000},
		},
		{
			name:     "8 bit stereo converts unsigned PCM per channel",
			channels: 2,
			bits:     8,
			pcm:      []byte{0, 255, 128, 64},
			want:     []int16{-32768, 32512, 0, -16384},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clip, err := decodeClip(wavPCM(test.channels, test.bits, 24_000, test.pcm), "clip.wav", 24_000)
			if err != nil {
				t.Fatalf("decodeClip() error = %v", err)
			}
			if clip.Rate != 24_000 {
				t.Errorf("Rate = %d, want 24000", clip.Rate)
			}
			if !reflect.DeepEqual(clip.Samples, test.want) {
				t.Errorf("Samples = %v, want %v", clip.Samples, test.want)
			}
		})
	}
}

func TestDecodeClipResamplesToTargetRate(t *testing.T) {
	clip, err := decodeClip(wavPCM(1, 16, 24_000, pcm16(0, 1000)), "clip.wav", 48_000)
	if err != nil {
		t.Fatalf("decodeClip() error = %v", err)
	}
	if clip.Rate != 48_000 {
		t.Errorf("Rate = %d, want 48000", clip.Rate)
	}
	want := []int16{0, 0, 500, 500, 1000, 1000, 1000, 1000}
	if !reflect.DeepEqual(clip.Samples, want) {
		t.Errorf("Samples = %v, want %v", clip.Samples, want)
	}
}

func TestDecodeWAVSkipsPaddedUnknownChunks(t *testing.T) {
	data := wavPCM(1, 16, 48_000, pcm16(1234))
	data = insertWAVChunk(data, 36, "JUNK", []byte{0x7F})

	clip, err := decodeWAV(data)
	if err != nil {
		t.Fatalf("decodeWAV() error = %v", err)
	}
	want := []int16{1234, 1234}
	if !reflect.DeepEqual(clip.Samples, want) {
		t.Errorf("Samples = %v, want %v", clip.Samples, want)
	}
}

func TestDecodeWAVRejectsUnsupportedPCMLayouts(t *testing.T) {
	tests := []struct {
		name     string
		channels uint16
		bits     uint16
		pcm      []byte
	}{
		{name: "24 bit", channels: 1, bits: 24, pcm: []byte{0, 0, 0}},
		{name: "three channels", channels: 3, bits: 16, pcm: pcm16(0, 0, 0)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeWAV(wavPCM(test.channels, test.bits, 48_000, test.pcm)); err == nil {
				t.Fatal("decodeWAV() succeeded for an unsupported PCM layout")
			}
		})
	}
}

func TestDecodeWAVRejectsMissingData(t *testing.T) {
	_, err := decodeWAV(wavPCM(1, 16, 48_000, nil))
	if err == nil {
		t.Fatal("decodeWAV() succeeded for a WAV without a data chunk")
	}
}

func pcm16(samples ...int16) []byte {
	data := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(sample))
	}
	return data
}

func insertWAVChunk(data []byte, offset int, id string, payload []byte) []byte {
	padding := len(payload) % 2
	chunk := make([]byte, 8+len(payload)+padding)
	copy(chunk[:4], id)
	binary.LittleEndian.PutUint32(chunk[4:8], uint32(len(payload)))
	copy(chunk[8:], payload)
	data = append(data[:offset:offset], append(chunk, data[offset:]...)...)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	return data
}

func wavPCM(channels, bits uint16, rate uint32, pcm []byte) []byte {
	fmtChunkSize := 16
	dataChunkSize := len(pcm)
	dataPadding := dataChunkSize % 2
	data := make([]byte, 12+8+fmtChunkSize+8+dataChunkSize+dataPadding)
	copy(data[:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], uint32(fmtChunkSize))
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], channels)
	binary.LittleEndian.PutUint32(data[24:28], rate)
	blockAlign := channels * (bits / 8)
	binary.LittleEndian.PutUint32(data[28:32], rate*uint32(blockAlign))
	binary.LittleEndian.PutUint16(data[32:34], blockAlign)
	binary.LittleEndian.PutUint16(data[34:36], bits)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(dataChunkSize))
	copy(data[44:], pcm)
	return data
}

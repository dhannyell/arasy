//go:build windows && amd64

package native

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/dhannyell/frameforge/math2d"
	mp3Decoder "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

type audioType uint8

const (
	unknown audioType = iota
	mp3
	wav
	ogg
)

type clipPCM struct {
	Samples []int16
	Rate    int
}

func decodeClip(data []byte, path string, targetRate int) (*clipPCM, error) {
	if len(data) == 0 {
		return nil, errors.New("native/audio: empty data")
	}
	format := detectFormat(data, path)
	if format == unknown {
		return nil, fmt.Errorf("native/audio: unknown format for: %q", path)
	}

	var clip *clipPCM
	var err error

	switch format {
	case wav:
		clip, err = decodeWAV(data)
	case mp3:
		clip, err = decodeMP3(data)
	case ogg:
		clip, err = decodeOGG(data)
	}

	if err != nil {
		return nil, fmt.Errorf("native/audio: format %d decode: %w", format, err)
	}
	if clip.Rate != targetRate {
		clip = resampleLinear(clip, targetRate)
	}

	return clip, nil
}

func detectFormat(data []byte, path string) audioType {
	if len(data) < 4 {
		return unknown
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".wav":
		return wav
	case ".mp3":
		return mp3
	case ".ogg":
		return ogg
	}
	switch {
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WAVE":
		return wav
	case data[0] == 0xFF && (data[1]&0xF0) == 0xF0:
		return mp3
	case bytes.HasPrefix(data, []byte("OggS")):
		return ogg
	}
	return unknown
}

type wavSchema struct {
	audioFormat   uint16
	numChannels   uint16
	sampleRate    uint32
	byteRate      uint32
	blockAlign    uint16
	bitsPerSample uint16
}

// --- WAV ---

func decodeWAV(data []byte) (*clipPCM, error) {
	if len(data) < 12 {
		return nil, errors.New("native/audio: truncated RIFF header")
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("native/audio: not a RIFF/WAVE file")
	}
	r := bytes.NewReader(data[12:])
	var wavFmt wavSchema
	haveFmt := false
	var pcmBuf []byte
	for r.Len() >= 8 {
		var chunkID [4]byte
		if _, err := io.ReadFull(r, chunkID[:]); err != nil {
			return nil, fmt.Errorf("native/audio: read chunk ID: %w", err)
		}
		var chunkSize uint32
		if err := binary.Read(r, binary.LittleEndian, &chunkSize); err != nil {
			return nil, fmt.Errorf("native/audio: read chunk size: %w", err)
		}
		switch string(chunkID[:]) {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("native/audio: chunk size too small: %d", chunkSize)
			}
			if err := binary.Read(r, binary.LittleEndian, &wavFmt.audioFormat); err != nil {
				return nil, fmt.Errorf("native/audio: read audio format: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &wavFmt.numChannels); err != nil {
				return nil, fmt.Errorf("read channels: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &wavFmt.sampleRate); err != nil {
				return nil, fmt.Errorf("read sample rate: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &wavFmt.byteRate); err != nil {
				return nil, fmt.Errorf("read byte rate: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &wavFmt.blockAlign); err != nil {
				return nil, fmt.Errorf("read block align: %w", err)
			}
			if err := binary.Read(r, binary.LittleEndian, &wavFmt.bitsPerSample); err != nil {
				return nil, fmt.Errorf("read bits per sample: %w", err)
			}
			haveFmt = true
			if err := skipWAVChunk(r, chunkSize-16, chunkSize); err != nil {
				return nil, err
			}
		case "data":
			pcmBuf = make([]byte, chunkSize)
			if _, err := io.ReadFull(r, pcmBuf); err != nil {
				return nil, fmt.Errorf("native/audio: read PCM data: %w", err)
			}
			if err := skipWAVPadding(r, chunkSize); err != nil {
				return nil, err
			}
		default:
			if err := skipWAVChunk(r, chunkSize, chunkSize); err != nil {
				return nil, err
			}
		}
	}
	if !haveFmt {
		return nil, errors.New("native/audio: missing fmt chunk")
	}
	if len(pcmBuf) == 0 {
		return nil, errors.New("native/audio: missing data chunk")
	}
	if wavFmt.audioFormat != 1 {
		return nil, fmt.Errorf("native/audio: unsupported audio format %d (only PCM=1)", wavFmt.audioFormat)
	}
	return convertPCM(pcmBuf, &wavFmt)
}

func skipWAVChunk(r *bytes.Reader, payloadSize, chunkSize uint32) error {
	if int64(payloadSize) > int64(r.Len()) {
		return errors.New("native/audio: truncated chunk")
	}
	if _, err := r.Seek(int64(payloadSize), io.SeekCurrent); err != nil {
		return fmt.Errorf("native/audio: seek: %w", err)
	}
	return skipWAVPadding(r, chunkSize)
}

func skipWAVPadding(r *bytes.Reader, chunkSize uint32) error {
	if chunkSize%2 == 0 {
		return nil
	}
	if _, err := r.ReadByte(); err != nil {
		return fmt.Errorf("native/audio: read chunk padding: %w", err)
	}
	return nil
}

func convertPCM(data []byte, schema *wavSchema) (*clipPCM, error) {
	if schema.numChannels != 1 && schema.numChannels != 2 {
		return nil, fmt.Errorf("native/audio: unsupported channel count %d", schema.numChannels)
	}
	if schema.bitsPerSample != 8 && schema.bitsPerSample != 16 {
		return nil, fmt.Errorf("native/audio: unsupported bits per sample %d", schema.bitsPerSample)
	}
	bytesPerSample := schema.bitsPerSample / 8
	frameBytes := bytesPerSample * uint16(schema.numChannels)
	if len(data)%int(frameBytes) != 0 {
		return nil, errors.New("native/audio: truncated PCM frame")
	}
	frameCount := len(data) / int(frameBytes)

	switch {
	case schema.numChannels == 1 && schema.bitsPerSample == 16:
		samples := make([]int16, frameCount*2)
		for i := range frameCount {
			s := int16(binary.LittleEndian.Uint16(data[i*2:]))
			samples[i*2] = s
			samples[i*2+1] = s
		}
		return &clipPCM{Rate: int(schema.sampleRate), Samples: samples}, nil
	case schema.numChannels == 1 && schema.bitsPerSample == 8:
		samples := make([]int16, frameCount*2)
		for i := range frameCount {
			s := (int16(data[i]) - 128) << 8
			samples[i*2] = s
			samples[i*2+1] = s
		}
		return &clipPCM{Rate: int(schema.sampleRate), Samples: samples}, nil
	case schema.numChannels == 2 && schema.bitsPerSample == 16:
		if len(data) > 0 {
			samples := unsafe.Slice((*int16)(unsafe.Pointer(&data[0])), len(data)/2)
			return &clipPCM{Rate: int(schema.sampleRate), Samples: samples}, nil
		}
		return &clipPCM{Rate: int(schema.sampleRate)}, nil
	case schema.numChannels == 2 && schema.bitsPerSample == 8:
		samples := make([]int16, frameCount*2)
		for i := range frameCount {
			samples[i*2] = (int16(data[i*2]) - 128) << 8
			samples[i*2+1] = (int16(data[i*2+1]) - 128) << 8
		}
		return &clipPCM{Rate: int(schema.sampleRate), Samples: samples}, nil
	default:
		return nil, errors.New("native/audio: unsupported PCM layout")
	}
}

// --- MP3 ---

func decodeMP3(data []byte) (*clipPCM, error) {
	dec, err := mp3Decoder.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("native/audio: create decoder %w", err)
	}
	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, fmt.Errorf("native/audio: read PCM %w", err)
	}
	var samples []int16
	if len(raw) > 0 {
		samples = unsafe.Slice((*int16)(unsafe.Pointer(&raw[0])), len(raw)/2)
	}
	return &clipPCM{Samples: samples, Rate: dec.SampleRate()}, nil
}

// --- OGG Vorbis ---

func decodeOGG(data []byte) (*clipPCM, error) {
	floats, format, err := oggvorbis.ReadAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read all: %w", err)
	}
	frameCount := len(floats) / format.Channels

	switch format.Channels {
	case 1:
		samples := make([]int16, frameCount*2)
		for i := 0; i < frameCount; i++ {
			v := f32toI16(floats[i])
			samples[i*2] = v
			samples[i*2+1] = v
		}
		return &clipPCM{Samples: samples, Rate: format.SampleRate}, nil

	case 2:
		samples := make([]int16, len(floats))
		for i, f := range floats {
			samples[i] = f32toI16(f)
		}
		return &clipPCM{Samples: samples, Rate: format.SampleRate}, nil

	default:
		samples := make([]int16, frameCount*2)
		for i := 0; i < frameCount; i++ {
			base := i * format.Channels
			l := floats[base]
			r := floats[base+1]
			for c := 2; c < format.Channels; c++ {
				v := floats[base+c]
				l += v * 0.5
				r += v * 0.5
			}
			samples[i*2] = f32toI16(l)
			samples[i*2+1] = f32toI16(r)
		}
		return &clipPCM{Samples: samples, Rate: format.SampleRate}, nil
	}
}

func f32toI16(v float32) int16 {
	if v > 1 {
		return math.MaxInt16
	}
	if v < -1 {
		return math.MinInt16
	}
	return int16(v * math.MaxInt16)
}

// --- Resample -------

func resampleLinear(clip *clipPCM, targetRate int) *clipPCM {
	if clip.Rate == targetRate || len(clip.Samples) < 2 {
		return clip
	}
	ratio := float64(clip.Rate) / float64(targetRate)
	srcFrames := len(clip.Samples) / 2
	dstFrames := int(float64(srcFrames) / ratio)
	if dstFrames <= 0 {
		dstFrames = 1
	}
	out := make([]int16, dstFrames*2)
	for f := 0; f < dstFrames; f++ {
		pos := float64(f) * ratio
		srcIdx := int(pos)
		frac := pos - float64(srcIdx)
		nextIdx := srcIdx + 1
		if nextIdx >= srcFrames {
			nextIdx = srcIdx
		}
		ls := float64(clip.Samples[srcIdx*2])
		rs := float64(clip.Samples[srcIdx*2+1])
		le := float64(clip.Samples[nextIdx*2])
		re := float64(clip.Samples[nextIdx*2+1])
		out[f*2] = int16(math2d.Lerp(ls, le, frac))
		out[f*2+1] = int16(math2d.Lerp(rs, re, frac))
	}
	return &clipPCM{Rate: targetRate, Samples: out}
}

package main

import (
	"encoding/binary"
	"flag"
	"math"
	"os"
	"path/filepath"
)

const sampleRate = 44100

func main() {
	out := flag.String("out", "", "output directory")
	flag.Parse()
	if *out == "" {
		panic("-out is required")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}

	mustWrite(filepath.Join(*out, "paddle.wav"), tone(440, 0.08, 0.8))
	mustWrite(filepath.Join(*out, "brick.wav"), tone(880, 0.06, 0.7))
	mustWrite(filepath.Join(*out, "lost.wav"), sweep(300, 120, 0.35, 0.75))
	mustWrite(filepath.Join(*out, "music.wav"), arpeggio())
}

func mustWrite(path string, samples []int16) {
	if err := writeWAV(path, samples); err != nil {
		panic(err)
	}
}

func tone(freq, seconds, gain float64) []int16 {
	n := int(seconds * sampleRate)
	samples := make([]int16, n)
	for i := range samples {
		t := float64(i) / sampleRate
		env := math.Exp(-7 * t / seconds)
		samples[i] = sample(math.Sin(2*math.Pi*freq*t) * gain * env)
	}
	return samples
}

func sweep(startFreq, endFreq, seconds, gain float64) []int16 {
	n := int(seconds * sampleRate)
	samples := make([]int16, n)
	phase := 0.0
	for i := range samples {
		p := float64(i) / float64(n)
		freq := startFreq + (endFreq-startFreq)*p
		phase += 2 * math.Pi * freq / sampleRate
		env := math.Exp(-4 * p)
		samples[i] = sample(math.Sin(phase) * gain * env)
	}
	return samples
}

func arpeggio() []int16 {
	notes := []float64{220, 277.18, 329.63, 440, 329.63, 277.18, 220, 164.81}
	noteLen := sampleRate / 4
	samples := make([]int16, noteLen*len(notes))
	for noteIdx, freq := range notes {
		for i := 0; i < noteLen; i++ {
			p := float64(i) / float64(noteLen)
			env := math.Sin(math.Pi * p)
			t := float64(i) / sampleRate
			samples[noteIdx*noteLen+i] = sample(math.Sin(2*math.Pi*freq*t) * 0.3 * env)
		}
	}
	return samples
}

func sample(v float64) int16 {
	if v > 1 {
		v = 1
	}
	if v < -1 {
		v = -1
	}
	return int16(v * math.MaxInt16)
}

func writeWAV(path string, samples []int16) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dataBytes := uint32(len(samples) * 2)
	if _, err := f.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36)+dataBytes); err != nil {
		return err
	}
	if _, err := f.Write([]byte("WAVEfmt ")); err != nil {
		return err
	}
	fields := []any{
		uint32(16),
		uint16(1),
		uint16(1),
		uint32(sampleRate),
		uint32(sampleRate * 2),
		uint16(2),
		uint16(16),
	}
	for _, field := range fields {
		if err := binary.Write(f, binary.LittleEndian, field); err != nil {
			return err
		}
	}
	if _, err := f.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, dataBytes); err != nil {
		return err
	}
	return binary.Write(f, binary.LittleEndian, samples)
}

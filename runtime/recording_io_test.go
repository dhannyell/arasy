package runtime

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/input"
)

func TestRecordingWriteToReadRecordingRoundTrip(t *testing.T) {
	rec := Recording{
		Ticks: 120,
		Seed:  42,
		Anchors: []Anchor{
			{Tick: 0, Checksum: 11},
			{Tick: 60, Checksum: 22},
		},
		Events: []input.Event{
			{
				Tick:   3,
				Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
				Kind:   input.EventButtonDown,
			},
			{
				Tick:   4,
				Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelY},
				Kind:   input.EventAxisChanged,
				Value:  -2,
				X:      10,
				Y:      20,
			},
		},
	}

	var buf bytes.Buffer
	n, err := rec.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("WriteTo bytes = %d, buffer len = %d", n, buf.Len())
	}

	got, err := ReadRecording(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadRecording returned error: %v", err)
	}
	if !reflect.DeepEqual(got, rec) {
		t.Fatalf("round trip = %#v, want %#v", got, rec)
	}
}

func TestReadRecordingRejectsInvalidMagic(t *testing.T) {
	_, err := ReadRecording(strings.NewReader("NOPE"))
	if !errors.Is(err, ErrNotARecording) {
		t.Fatalf("error = %v, want ErrNotARecording", err)
	}
}

func TestReadRecordingRejectsUnsupportedVersion(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(recordingMagic)
	if err := binary.Write(&buf, binary.LittleEndian, uint16(999)); err != nil {
		t.Fatal(err)
	}

	_, err := ReadRecording(bytes.NewReader(buf.Bytes()))
	if !errors.Is(err, ErrUnsupportedRecordingFormat) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordingFormat", err)
	}
}

func TestVerifyRecordingChecksAnchors(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())
	driver.StartRecording()

	for tick := range 90 {
		kind := input.EventButtonUp
		if tick%20 < 10 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
	}

	rec := driver.StopRecording()
	if len(rec.Anchors) == 0 {
		t.Fatal("recording captured no anchors")
	}
	if err := VerifyRecording(&inputBackGame{}, slog.Default(), rec); err != nil {
		t.Fatalf("VerifyRecording returned error: %v", err)
	}

	rec.Anchors[len(rec.Anchors)-1].Checksum++
	err := VerifyRecording(&inputBackGame{}, slog.Default(), rec)
	if err == nil {
		t.Fatal("VerifyRecording succeeded with tampered anchor")
	}
	if !strings.Contains(err.Error(), "anchor mismatch") {
		t.Fatalf("error = %v, want anchor mismatch", err)
	}
}

func TestReadRecordingRejectsInvalidEvent(t *testing.T) {
	rec := Recording{
		Ticks: 1,
		Events: []input.Event{{
			Tick:   0,
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.SourceCode(65535)},
			Kind:   input.EventButtonDown,
		}},
	}

	var buf bytes.Buffer
	if _, err := rec.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	_, err := ReadRecording(bytes.NewReader(buf.Bytes()))
	if err == nil || !strings.Contains(err.Error(), "invalid event") {
		t.Fatalf("error = %v, want invalid event", err)
	}
}

package runtime

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

func TestRecordingWriteToReadRecordingRoundTrip(t *testing.T) {
	rec := Recording{
		Ticks:           120,
		Seed:            42,
		SimulationHz:    60,
		GamepadDeadzone: .25,
		ContentChecksum: 77,
		Viewport:        math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 1920, Y: 1080}),
		Viewports: []ViewportChange{
			{Tick: 30, Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})},
		},
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

func TestReadRecordingRejectsTrailingData(t *testing.T) {
	data, err := (Recording{Ticks: 1}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary returned error: %v", err)
	}
	data = append(data, 0)

	_, err = ReadRecordingFromBytes(data)
	if !errors.Is(err, ErrUnsupportedRecordingFormat) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordingFormat", err)
	}
}

func TestReadRecordingRejectsLegacyV1Layout(t *testing.T) {
	data, err := (Recording{
		Ticks:   30,
		Anchors: []Anchor{{Tick: 30, Checksum: 1}},
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary returned error: %v", err)
	}

	// Before GamepadDeadzone was added, v1 placed Viewport immediately after
	// Seed. Removing that field reconstructs the incompatible legacy layout.
	const deadzoneOffset = len(recordingMagic) + 2 + 8 + 8 + 2
	legacy := append([]byte(nil), data[:deadzoneOffset]...)
	legacy = append(legacy, data[deadzoneOffset+8:]...)

	if _, err := ReadRecordingFromBytes(legacy); err == nil {
		t.Fatal("ReadRecordingFromBytes accepted the legacy v1 layout")
	}
}

// TestReadRecordingDoesNotPreallocateFromHeaderCounts pins the fix for a
// header whose element counts are not backed by any payload. The counts are
// attacker-controlled on the untrusted-upload path, and sizing the slices from
// them let 66 bytes of header reserve hundreds of megabytes before the read
// failed. Parsing must stay proportional to the bytes actually present.
func TestReadRecordingDoesNotPreallocateFromHeaderCounts(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(recordingMagic)
	for _, field := range []any{
		recordingVersion,
		uint64(0), uint64(0), // ticks, seed
		uint16(0),  // simulation hz
		float64(0), // gamepad deadzone
		uint64(0),  // content checksum
		float64(0), float64(0), float64(0), float64(0), // viewport
		uint32(maxRecordingAnchors),
		uint32(maxRecordingEvents),
		uint32(maxRecordingAnchors),
	} {
		if err := binary.Write(&buf, binary.LittleEndian, field); err != nil {
			t.Fatal(err)
		}
	}
	data := buf.Bytes()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := ReadRecordingFromBytes(data)
	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("expected an error for a header with no payload")
	}
	// Generous: the honest cost is a few kilobytes. The pre-fix cost was
	// ~400 MB, so this catches a regression by three orders of magnitude.
	const budget = 1 << 20
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > budget {
		t.Fatalf("parsing %d bytes allocated %d bytes, want at most %d",
			len(data), allocated, budget)
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

// TestRecordingCapturesViewportForUIHitTesting is the regression test for the
// breakout .arrp replay bug: a recording made on a platform surface different
// from the game's Config default viewport (e.g. a responsive browser canvas)
// must capture and restore that viewport, or a UI button click that started
// the game live is missed during replay — the game stays frozen in its menu
// state and the first checksum anchor diverges with "anchor mismatch".
//
// uiClickGame is the same minimal stand-in used by the scrub viewport tests:
// its state transitions on a UI button click whose hit-test depends on the
// viewport (button at reference 40-60, 40-60 in SpaceRef).
func TestRecordingCapturesViewportForUIHitTesting(t *testing.T) {
	game := &uiClickGame{}
	driver := NewDriver(game, slog.Default())

	// Platform surface (200×200) differs from the Config default (100×100),
	// just like a responsive browser canvas. Screen (100,100) maps to
	// reference (50,50) at 200×200, hitting the button; at 100×100 it maps to
	// (100,100), missing it.
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 200}))

	driver.StartRecording()
	driver.ApplyInput([]input.Event{mouseDownAt(100, 100), mouseUpAt(100, 100)})
	driver.Step()
	if !game.state {
		t.Fatal("live click did not start the game — test setup is wrong")
	}

	// Run past the first anchor (tick 30, default KeyframeInterval 30).
	for range 35 {
		driver.Step()
	}

	rec := driver.StopRecording()
	if rec.Viewport.Width() != 200 || rec.Viewport.Height() != 200 {
		t.Fatalf("recording viewport = %v×%v, want 200×200", rec.Viewport.Width(), rec.Viewport.Height())
	}

	// The recording must verify: the viewport is restored, the click hits,
	// the game starts, and every anchor matches.
	if err := VerifyRecording(&uiClickGame{}, slog.Default(), rec); err != nil {
		t.Fatalf("VerifyRecording returned error: %v", err)
	}

	// Round-trip through the binary format must preserve the viewport.
	var buf bytes.Buffer
	if _, err := rec.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	rt, err := ReadRecording(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadRecording returned error: %v", err)
	}
	if err := VerifyRecording(&uiClickGame{}, slog.Default(), rt); err != nil {
		t.Fatalf("VerifyRecording after round-trip returned error: %v", err)
	}

	// Stripping the viewport must break verification — without it, the replay
	// uses the Config default (100×100), the click misses, the game freezes,
	// and the first anchor (tick 30) diverges. This is the bug the user hit:
	// "anchor mismatch at tick 90: got <frozen> want <live>".
	strip := rt
	strip.Viewport = math2d.Rect{}
	strip.Viewports = nil
	err = VerifyRecording(&uiClickGame{}, slog.Default(), strip)
	if err == nil {
		t.Fatal("VerifyRecording succeeded without viewport — expected anchor mismatch")
	}
	if !strings.Contains(err.Error(), "anchor mismatch") {
		t.Fatalf("error = %v, want anchor mismatch", err)
	}
}

// TestRecordingMarshalBinaryRoundTrip verifies the byte-oriented helpers used
// by platforms that don't have a filesystem (e.g. the WASM host, which loads
// bytes through the browser File API).
func TestRecordingMarshalBinaryRoundTrip(t *testing.T) {
	rec := Recording{
		Ticks:    60,
		Seed:     7,
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}),
		Viewports: []ViewportChange{
			{Tick: 30, Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})},
		},
		Anchors: []Anchor{{Tick: 0, Checksum: 99}, {Tick: 30, Checksum: 100}},
		Events: []input.Event{
			{Tick: 1, Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeySpace}, Kind: input.EventButtonDown},
		},
	}

	data, err := rec.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary returned error: %v", err)
	}

	got, err := ReadRecordingFromBytes(data)
	if err != nil {
		t.Fatalf("ReadRecordingFromBytes returned error: %v", err)
	}
	if !reflect.DeepEqual(got, rec) {
		t.Fatalf("byte round trip = %#v, want %#v", got, rec)
	}
}

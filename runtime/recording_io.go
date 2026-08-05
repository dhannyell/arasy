package runtime

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

var (
	ErrNotARecording              = errors.New("not an arasy recording")
	ErrUnsupportedRecordingFormat = errors.New("unsupported arasy recording format")
)

const (
	recordingMagic             = "ARRP"
	recordingVersion    uint16 = 1
	maxRecordingEvents         = 10_000_000
	maxRecordingAnchors        = 1_000_000
	// maxRecordingPrealloc bounds the capacity reserved up front for the
	// element slices. The counts in the header are attacker-controlled for a
	// file loaded from an untrusted source (ReadRecordingFromBytes is the
	// browser upload path), and they are not redundant with the payload
	// length, so sizing a slice from a count lets a few dozen bytes of header
	// reserve hundreds of megabytes. The slices grow as elements actually
	// decode instead; a real recording pays a handful of amortized regrowths.
	maxRecordingPrealloc = 1024
)

// initialCap returns the capacity to reserve for a header-declared count.
func initialCap(count uint32) int {
	return min(int(count), maxRecordingPrealloc)
}

// WriteTo serializes the recording to a versioned little-endian binary stream.
func (r Recording) WriteTo(w io.Writer) (int64, error) {
	cw := countingWriter{w: w}
	if err := cw.writeBytes([]byte(recordingMagic)); err != nil {
		return cw.n, err
	}
	if err := cw.write(recordingVersion); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.Ticks); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.Seed); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.SimulationHz); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.GamepadDeadzone); err != nil {
		return cw.n, err
	}
	// Content fingerprint at tick 0. Added in place without a version bump:
	// no .arrp is distributed yet, so committed fixtures regenerate instead.
	if err := cw.write(r.ContentChecksum); err != nil {
		return cw.n, err
	}
	// Viewport at tick 0 (4 float64 = 32 bytes). A zero rect signals "use the
	// game's Config default" — recordings made before viewport capture.
	if err := cw.write(r.Viewport.Min.X); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.Viewport.Min.Y); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.Viewport.Max.X); err != nil {
		return cw.n, err
	}
	if err := cw.write(r.Viewport.Max.Y); err != nil {
		return cw.n, err
	}
	if len(r.Anchors) > maxRecordingAnchors {
		return cw.n, fmt.Errorf("recording has %d anchors, max %d", len(r.Anchors), maxRecordingAnchors)
	}
	if len(r.Events) > maxRecordingEvents {
		return cw.n, fmt.Errorf("recording has %d events, max %d", len(r.Events), maxRecordingEvents)
	}
	if len(r.Viewports) > maxRecordingAnchors {
		return cw.n, fmt.Errorf("recording has %d viewport changes, max %d", len(r.Viewports), maxRecordingAnchors)
	}
	if err := cw.write(uint32(len(r.Anchors))); err != nil {
		return cw.n, err
	}
	if err := cw.write(uint32(len(r.Events))); err != nil {
		return cw.n, err
	}
	if err := cw.write(uint32(len(r.Viewports))); err != nil {
		return cw.n, err
	}
	for _, anchor := range r.Anchors {
		if err := cw.write(anchor.Tick); err != nil {
			return cw.n, err
		}
		if err := cw.write(anchor.Checksum); err != nil {
			return cw.n, err
		}
	}
	for _, event := range r.Events {
		if err := cw.write(event.Tick); err != nil {
			return cw.n, err
		}
		if err := cw.write(uint8(event.Source.Device)); err != nil {
			return cw.n, err
		}
		if err := cw.write(uint16(event.Source.Code)); err != nil {
			return cw.n, err
		}
		if err := cw.write(uint8(event.Kind)); err != nil {
			return cw.n, err
		}
		if err := cw.write(event.Value); err != nil {
			return cw.n, err
		}
		if err := cw.write(event.X); err != nil {
			return cw.n, err
		}
		if err := cw.write(event.Y); err != nil {
			return cw.n, err
		}
	}
	for _, vc := range r.Viewports {
		if err := cw.write(vc.Tick); err != nil {
			return cw.n, err
		}
		if err := cw.write(vc.Viewport.Min.X); err != nil {
			return cw.n, err
		}
		if err := cw.write(vc.Viewport.Min.Y); err != nil {
			return cw.n, err
		}
		if err := cw.write(vc.Viewport.Max.X); err != nil {
			return cw.n, err
		}
		if err := cw.write(vc.Viewport.Max.Y); err != nil {
			return cw.n, err
		}
	}
	return cw.n, nil
}

// ReadRecording reads a versioned little-endian recording stream.
func ReadRecording(r io.Reader) (Recording, error) {
	cr := countingReader{r: r}
	magic := make([]byte, len(recordingMagic))
	if err := cr.readBytes(magic); err != nil {
		return Recording{}, err
	}
	if string(magic) != recordingMagic {
		return Recording{}, ErrNotARecording
	}

	var version uint16
	if err := cr.read(&version); err != nil {
		return Recording{}, err
	}
	if version != recordingVersion {
		return Recording{}, fmt.Errorf("%w: %d", ErrUnsupportedRecordingFormat, version)
	}

	var rec Recording
	if err := cr.read(&rec.Ticks); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.Seed); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.SimulationHz); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.GamepadDeadzone); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.ContentChecksum); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.Viewport.Min.X); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.Viewport.Min.Y); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.Viewport.Max.X); err != nil {
		return Recording{}, err
	}
	if err := cr.read(&rec.Viewport.Max.Y); err != nil {
		return Recording{}, err
	}
	var anchorCount uint32
	if err := cr.read(&anchorCount); err != nil {
		return Recording{}, err
	}
	if anchorCount > maxRecordingAnchors {
		return Recording{}, fmt.Errorf("recording has %d anchors, max %d", anchorCount, maxRecordingAnchors)
	}
	var eventCount uint32
	if err := cr.read(&eventCount); err != nil {
		return Recording{}, err
	}
	if eventCount > maxRecordingEvents {
		return Recording{}, fmt.Errorf("recording has %d events, max %d", eventCount, maxRecordingEvents)
	}
	var viewportCount uint32
	if err := cr.read(&viewportCount); err != nil {
		return Recording{}, err
	}
	if viewportCount > maxRecordingAnchors {
		return Recording{}, fmt.Errorf("recording has %d viewport changes, max %d", viewportCount, maxRecordingAnchors)
	}

	rec.Anchors = make([]Anchor, 0, initialCap(anchorCount))
	for range anchorCount {
		var anchor Anchor
		if err := cr.read(&anchor.Tick); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&anchor.Checksum); err != nil {
			return Recording{}, err
		}
		rec.Anchors = append(rec.Anchors, anchor)
	}

	rec.Events = make([]input.Event, 0, initialCap(eventCount))
	for i := range eventCount {
		var event input.Event
		var device uint8
		var code uint16
		var kind uint8
		if err := cr.read(&event.Tick); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&device); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&code); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&kind); err != nil {
			return Recording{}, err
		}
		event.Source = input.Source{Device: input.DeviceKind(device), Code: input.SourceCode(code)}
		event.Kind = input.EventKind(kind)
		if err := cr.read(&event.Value); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&event.X); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&event.Y); err != nil {
			return Recording{}, err
		}
		if !event.Valid() {
			return Recording{}, fmt.Errorf("invalid event at index %d", i)
		}
		rec.Events = append(rec.Events, event)
	}

	rec.Viewports = make([]ViewportChange, 0, initialCap(viewportCount))
	for range viewportCount {
		var change ViewportChange
		if err := cr.read(&change.Tick); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&change.Viewport.Min.X); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&change.Viewport.Min.Y); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&change.Viewport.Max.X); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&change.Viewport.Max.Y); err != nil {
			return Recording{}, err
		}
		rec.Viewports = append(rec.Viewports, change)
	}

	var trailing byte
	if err := cr.read(&trailing); err == nil {
		return Recording{}, fmt.Errorf("%w: trailing data after recording", ErrUnsupportedRecordingFormat)
	} else if !errors.Is(err, io.EOF) {
		return Recording{}, err
	}

	return rec, nil
}

// MarshalBinary serializes the recording to its ARRP binary form as a byte
// slice. It is the byte-oriented convenience over WriteTo for platforms that
// obtain deliver bytes through a non-file channel (e.g. a browser File API
// download). The format is identical to WriteTo; see WriteTo for the layout.
func (r Recording) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReadRecordingFromBytes parses an ARRP byte slice. It is the byte-oriented
// convenience over ReadRecording for platforms that load bytes through a
// non-file channel (e.g. a browser File API upload).
func ReadRecordingFromBytes(data []byte) (Recording, error) {
	return ReadRecording(bytes.NewReader(data))
}

// VerifyRecording replays a recording and checks each stored checksum anchor.
func VerifyRecording(game Game, log *slog.Logger, rec Recording) error {
	if log == nil {
		log = slog.Default()
	}
	driver := newDriverFromRecording(game, log, rec)
	driver.recorder = nil

	// Restore the surface size active at tick 0 so viewport-dependent logic
	// (e.g. a UI button hit-test) matches the live run. Without this, pointer
	// coordinates captured in a different surface size map to the wrong
	// reference-canvas position and a click that started the game live is
	// missed during replay — the checksum freezes and anchors diverge.
	if rec.Viewport.Width() > 0 && rec.Viewport.Height() > 0 {
		driver.viewport = rec.Viewport
	}

	viewportByTick := make(map[uint64]math2d.Rect, len(rec.Viewports))
	for _, vc := range rec.Viewports {
		if vc.Tick < rec.Ticks {
			viewportByTick[vc.Tick] = vc.Viewport
		}
	}

	eventsByTick := make(map[uint64][]input.Event, len(rec.Events))
	for _, event := range rec.Events {
		eventsByTick[event.Tick] = append(eventsByTick[event.Tick], event)
	}

	warnOnContentDivergence(driver, log, rec)

	nextAnchor := 0
	for nextAnchor < len(rec.Anchors) && rec.Anchors[nextAnchor].Tick == 0 {
		if got, want := driver.world.SimChecksum(), rec.Anchors[nextAnchor].Checksum; got != want {
			return fmt.Errorf("anchor mismatch at tick 0: got %d want %d", got, want)
		}
		nextAnchor++
	}
	for tick := uint64(0); tick < rec.Ticks; tick++ {
		// Match live order: SetViewport, ApplyInput, Step.
		if vp, ok := viewportByTick[tick]; ok {
			driver.viewport = vp
		}
		if events := eventsByTick[tick]; len(events) > 0 {
			driver.ApplyInput(events)
		}
		driver.Step()
		for nextAnchor < len(rec.Anchors) && rec.Anchors[nextAnchor].Tick == driver.tick {
			if got, want := driver.world.SimChecksum(), rec.Anchors[nextAnchor].Checksum; got != want {
				return fmt.Errorf("anchor mismatch at tick %d: got %d want %d", driver.tick, got, want)
			}
			nextAnchor++
		}
	}
	if nextAnchor != len(rec.Anchors) {
		return fmt.Errorf("anchor at tick %d beyond recording length %d", rec.Anchors[nextAnchor].Tick, rec.Ticks)
	}
	return nil
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) write(v any) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func (w *countingWriter) writeBytes(data []byte) error {
	_, err := w.Write(data)
	return err
}

func (w *countingWriter) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	w.n += int64(n)
	return n, err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) read(v any) error {
	return binary.Read(r, binary.LittleEndian, v)
}

func (r *countingReader) readBytes(data []byte) error {
	_, err := io.ReadFull(r, data)
	return err
}

func (r *countingReader) Read(data []byte) (int, error) {
	n, err := r.r.Read(data)
	r.n += int64(n)
	return n, err
}

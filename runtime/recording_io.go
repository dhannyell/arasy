package runtime

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/dhannyell/frameforge/input"
)

var (
	ErrNotARecording              = errors.New("not a frameforge recording")
	ErrUnsupportedRecordingFormat = errors.New("unsupported frameforge recording format")
)

const (
	recordingMagic             = "FFRP"
	recordingVersion    uint16 = 1
	maxRecordingEvents         = 10_000_000
	maxRecordingAnchors        = 1_000_000
)

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
	if len(r.Anchors) > maxRecordingAnchors {
		return cw.n, fmt.Errorf("recording has %d anchors, max %d", len(r.Anchors), maxRecordingAnchors)
	}
	if len(r.Events) > maxRecordingEvents {
		return cw.n, fmt.Errorf("recording has %d events, max %d", len(r.Events), maxRecordingEvents)
	}
	if err := cw.write(uint32(len(r.Anchors))); err != nil {
		return cw.n, err
	}
	if err := cw.write(uint32(len(r.Events))); err != nil {
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

	rec.Anchors = make([]Anchor, anchorCount)
	for i := range rec.Anchors {
		if err := cr.read(&rec.Anchors[i].Tick); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&rec.Anchors[i].Checksum); err != nil {
			return Recording{}, err
		}
	}

	rec.Events = make([]input.Event, eventCount)
	for i := range rec.Events {
		var device uint8
		var code uint16
		var kind uint8
		if err := cr.read(&rec.Events[i].Tick); err != nil {
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
		rec.Events[i].Source = input.Source{Device: input.DeviceKind(device), Code: input.SourceCode(code)}
		rec.Events[i].Kind = input.EventKind(kind)
		if err := cr.read(&rec.Events[i].Value); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&rec.Events[i].X); err != nil {
			return Recording{}, err
		}
		if err := cr.read(&rec.Events[i].Y); err != nil {
			return Recording{}, err
		}
		if !rec.Events[i].Valid() {
			return Recording{}, fmt.Errorf("invalid event at index %d", i)
		}
	}

	return rec, nil
}

// VerifyRecording replays a recording and checks each stored checksum anchor.
func VerifyRecording(game Game, log *slog.Logger, rec Recording) error {
	if log == nil {
		log = slog.Default()
	}
	driver := newDriver(game, log, &rec.Seed)
	driver.recorder = nil

	eventsByTick := make(map[uint64][]input.Event, len(rec.Events))
	for _, event := range rec.Events {
		eventsByTick[event.Tick] = append(eventsByTick[event.Tick], event)
	}

	nextAnchor := 0
	for nextAnchor < len(rec.Anchors) && rec.Anchors[nextAnchor].Tick == 0 {
		if got, want := driver.Checksum(), rec.Anchors[nextAnchor].Checksum; got != want {
			return fmt.Errorf("anchor mismatch at tick 0: got %d want %d", got, want)
		}
		nextAnchor++
	}
	for tick := uint64(0); tick < rec.Ticks; tick++ {
		if events := eventsByTick[tick]; len(events) > 0 {
			driver.ApplyInput(events)
		}
		driver.Step()
		for nextAnchor < len(rec.Anchors) && rec.Anchors[nextAnchor].Tick == driver.tick {
			if got, want := driver.Checksum(), rec.Anchors[nextAnchor].Checksum; got != want {
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

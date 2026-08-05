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

// The ARRP format evolves under these rules. They outlive any single field;
// anyone touching this file inherits them.
//
//  1. Backward compatibility is mandatory, forward is not. The reader loads
//     every released version; the writer always writes the current one. A
//     new engine build reading a player's old .arrp is the case that pays.
//     An old build reading a new file is not promised.
//  2. After the first public release, any layout change bumps
//     recordingVersion. Editing the layout in place without a bump is a
//     dated exception, valid only while no .arrp exists outside this
//     repository. It expires at 0.1.0.
//  3. A released version's layout is frozen. Add a new version, never edit a
//     published one. The reader can only honor rule 1 if each version's
//     meaning is immutable.
//  4. Absence has meaning, documented on the field: a zero Viewport means
//     "use the Config default", a zero SimulationHz falls back to the game's
//     rate, an absent provenance section means "recorded without
//     provenance". Never "unknown, assume something".
//  5. Every supported version keeps a byte fixture under
//     runtime/testdata/arrp, read by a test. When a new version is born, the
//     old fixture stays: it is the living proof of rule 3.
//  6. Metadata never enters simulation. Nothing in the provenance section
//     touches a checksum, feeds Update, or changes what a replay reproduces.
//     Metadata explains; it does not decide.
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
	// maxProvenanceString bounds each provenance string; ids and versions
	// are short by nature. maxProvenanceBlob bounds the game-owned payload.
	maxProvenanceString = 1024
	maxProvenanceBlob   = 1 << 20
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
	// Provenance is optional by design: absence is a single zero byte, the
	// state of every committed fixture, so recorded bytes stay
	// machine-independent.
	if !r.Provenance.Present {
		if err := cw.write(uint8(0)); err != nil {
			return cw.n, err
		}
		return cw.n, nil
	}
	if err := cw.write(uint8(1)); err != nil {
		return cw.n, err
	}
	for _, s := range []string{
		r.Provenance.EngineVersion,
		r.Provenance.GameID,
		r.Provenance.GameVersion,
		r.Provenance.Platform,
	} {
		if len(s) > maxProvenanceString {
			return cw.n, fmt.Errorf("provenance string has %d bytes, max %d", len(s), maxProvenanceString)
		}
		if err := cw.write(uint16(len(s))); err != nil {
			return cw.n, err
		}
		if err := cw.writeBytes([]byte(s)); err != nil {
			return cw.n, err
		}
	}
	if len(r.Provenance.GameBlob) > maxProvenanceBlob {
		return cw.n, fmt.Errorf("provenance blob has %d bytes, max %d", len(r.Provenance.GameBlob), maxProvenanceBlob)
	}
	if err := cw.write(uint32(len(r.Provenance.GameBlob))); err != nil {
		return cw.n, err
	}
	if err := cw.writeBytes(r.Provenance.GameBlob); err != nil {
		return cw.n, err
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
	// Dispatch by version (format rule 1): today only v1 exists, so the
	// reads below are the v1 layout inline. The first post-release bump adds
	// a case here instead of restructuring the reader.
	switch version {
	case 1:
	default:
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

	var provenancePresent uint8
	if err := cr.read(&provenancePresent); err != nil {
		return Recording{}, err
	}
	switch provenancePresent {
	case 0:
		// Absence is valid: recorded without provenance (format rule 4).
	case 1:
		rec.Provenance.Present = true
		for _, dst := range []*string{
			&rec.Provenance.EngineVersion,
			&rec.Provenance.GameID,
			&rec.Provenance.GameVersion,
			&rec.Provenance.Platform,
		} {
			s, err := cr.readString(maxProvenanceString)
			if err != nil {
				return Recording{}, err
			}
			*dst = s
		}
		var blobLen uint32
		if err := cr.read(&blobLen); err != nil {
			return Recording{}, err
		}
		if blobLen > maxProvenanceBlob {
			return Recording{}, fmt.Errorf("provenance blob has %d bytes, max %d", blobLen, maxProvenanceBlob)
		}
		blob, err := cr.readSizedBytes(blobLen)
		if err != nil {
			return Recording{}, err
		}
		rec.Provenance.GameBlob = blob
	default:
		return Recording{}, fmt.Errorf("%w: invalid provenance marker %d", ErrUnsupportedRecordingFormat, provenancePresent)
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
			return fmt.Errorf("anchor mismatch at tick 0: got %d want %d\n  %s", got, want, rec.Provenance.note())
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
				return fmt.Errorf("anchor mismatch at tick %d: got %d want %d\n  %s", driver.tick, got, want, rec.Provenance.note())
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

// readString reads a uint16 length-prefixed string bounded by max. The bound
// keeps a hostile header from turning two bytes into a large allocation.
func (r *countingReader) readString(max int) (string, error) {
	var n uint16
	if err := r.read(&n); err != nil {
		return "", err
	}
	if int(n) > max {
		return "", fmt.Errorf("recording string has %d bytes, max %d", n, max)
	}
	buf := make([]byte, n)
	if err := r.readBytes(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readSizedBytes reads a byte payload of declared length without trusting
// that length for allocation: the buffer grows as bytes actually arrive, the
// same discipline the element slices follow.
func (r *countingReader) readSizedBytes(count uint32) ([]byte, error) {
	if count == 0 {
		return nil, nil
	}
	data := make([]byte, 0, initialCap(count))
	chunk := make([]byte, min(int(count), maxRecordingPrealloc))
	remaining := int(count)
	for remaining > 0 {
		n := min(remaining, len(chunk))
		if err := r.readBytes(chunk[:n]); err != nil {
			return nil, err
		}
		data = append(data, chunk[:n]...)
		remaining -= n
	}
	return data, nil
}

func (r *countingReader) Read(data []byte) (int, error) {
	n, err := r.r.Read(data)
	r.n += int64(n)
	return n, err
}

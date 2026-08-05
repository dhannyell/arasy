package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// TestRecordingFormatReadsV1Fixture pins the v1 byte layout against the
// committed fixture (format rules 3 and 5 in recording_io.go): a refactor
// that breaks reading already-written files fails here, not in a player's
// bug report. If this fails, the layout changed — before 0.1.0 that means
// regenerating the fixture on purpose; after, it means a version bump was
// skipped. Never edit the fixture bytes to make the test pass.
func TestRecordingFormatReadsV1Fixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "arrp", "v1.arrp"))
	if err != nil {
		t.Fatalf("read v1 fixture: %v", err)
	}
	got, err := ReadRecordingFromBytes(data)
	if err != nil {
		t.Fatalf("decode v1 fixture: %v", err)
	}
	if !reflect.DeepEqual(got, v1FixtureRecording()) {
		t.Fatalf("v1 fixture = %#v, want %#v", got, v1FixtureRecording())
	}
}

// v1FixtureRecording is the value serialized into testdata/arrp/v1.arrp. It
// fills every section — including provenance, with fixed strings so the
// bytes stay machine-independent — because an unexercised field is an
// unguarded offset.
func v1FixtureRecording() Recording {
	return Recording{
		Ticks:           60,
		Seed:            7,
		SimulationHz:    60,
		GamepadDeadzone: .25,
		ContentChecksum: 99,
		Viewport:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
		Viewports: []ViewportChange{
			{Tick: 30, Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 640, Y: 480})},
		},
		Anchors: []Anchor{{Tick: 0, Checksum: 11}, {Tick: 30, Checksum: 22}},
		Events: []input.Event{{
			Tick:   3,
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		}},
		Provenance: Provenance{
			Present:       true,
			EngineVersion: "v0.1.0",
			GameID:        "fixture-game",
			GameVersion:   "1.0.0",
			Platform:      "linux/amd64",
			GameBlob:      []byte("blob"),
		},
	}
}

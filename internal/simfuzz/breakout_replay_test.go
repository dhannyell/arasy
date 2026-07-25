package simfuzz

import (
	"bytes"
	"testing"

	"github.com/dhannyell/arasy/examples/breakout"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
)

const breakoutFuzzTicks = 120

// checkBreakoutReplayFidelity extends P2 through a real game and UI transition:
// a pointer click enters Ready, Space launches the ball, and fuzzed directional
// input drives gameplay before the recording is verified and replayed.
func checkBreakoutReplayFidelity(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	ticks := clamp(script.MaxTicks, 8, breakoutFuzzTicks)
	byTick := groupEventsByTick(withoutSpaceEvents(script.Events))

	game := &breakout.Breakout{}
	driver := runtime.NewDriver(game, quietLog)
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}))
	initial := driver.Checksum()
	driver.StartRecording()

	for tick := range ticks {
		events := append([]input.Event(nil), byTick[uint64(tick)]...)
		switch tick {
		case 0:
			events = append(events, breakoutPlayClick()...)
		case 1:
			events = append(events, input.Event{
				Source: input.Keyboard(input.KeySpace),
				Kind:   input.EventButtonDown,
			})
		case 2:
			events = append(events, input.Event{
				Source: input.Keyboard(input.KeySpace),
				Kind:   input.EventButtonUp,
			})
		}
		driver.ApplyInput(events)
		driver.Step()
	}

	live := driver.Checksum()
	if live == initial {
		t.Fatalf("breakout fuzz scenario did not change world state\nrepro: %s",
			dumpRepro(t, "FuzzReplayFidelity", data))
	}
	rec := driver.StopRecording()
	var buf bytes.Buffer
	if _, err := rec.WriteTo(&buf); err != nil {
		t.Fatalf("breakout recording write: %v\nrepro: %s",
			err, dumpRepro(t, "FuzzReplayFidelity", data))
	}
	decoded, err := runtime.ReadRecordingFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("breakout recording re-read: %v\nrepro: %s",
			err, dumpRepro(t, "FuzzReplayFidelity", data))
	}
	if err := runtime.VerifyRecording(&breakout.Breakout{}, quietLog, decoded); err != nil {
		t.Fatalf("breakout VerifyRecording: %v\nrepro: %s",
			err, dumpRepro(t, "FuzzReplayFidelity", data))
	}
	if got := runtime.Replay(&breakout.Breakout{}, quietLog, decoded).Checksum(); got != live {
		t.Fatalf("breakout replay checksum %d != live %d\nrepro: %s",
			got, live, dumpRepro(t, "FuzzReplayFidelity", data))
	}
}

func breakoutPlayClick() []input.Event {
	const playX, playY = 960.0, 573.75
	return []input.Event{
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
			Kind:   input.EventPointerMove,
			X:      playX,
			Y:      playY,
		},
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
			Kind:   input.EventButtonDown,
			X:      playX,
			Y:      playY,
		},
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
			Kind:   input.EventButtonUp,
			X:      playX,
			Y:      playY,
		},
	}
}

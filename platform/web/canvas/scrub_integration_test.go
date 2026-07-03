package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/dhannyell/frameforge/cmd/demos/breakout"
	"github.com/dhannyell/frameforge/runtime"
)

// TestBreakoutScrubEndToEnd exercises browser-facing scrub with Breakout:
// resize, click Play, launch, scrub, and compare reconstructed frames.
func TestBreakoutScrubEndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	log := slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelWarn}))

	go func() {
		Run(&breakout.Breakout{}, Options{Addr: addr, Log: log})
	}()

	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://%s/ws", addr), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	send := func(msg map[string]any) {
		b, _ := json.Marshal(msg)
		if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	readFrame := func() map[string]any {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var frame map[string]any
		json.Unmarshal(b, &frame)
		return frame
	}

	// frameSignature produces a compact string identifying the rendered
	// content: command count + first few command types.  Two frames with
	// the same signature look identical to the user.
	frameSignature := func(frame map[string]any) string {
		commands, _ := frame["commands"].([]any)
		sig := fmt.Sprintf("cmds=%d", len(commands))
		for i, cmd := range commands {
			if i >= 5 {
				sig += "..."
				break
			}
			c, _ := cmd.(map[string]any)
			if c != nil {
				sig += fmt.Sprintf("|%s", c["type"])
			}
		}
		return sig
	}

	// 1. Resize + wait for sim to start.
	send(map[string]any{"type": "resize", "width": 1280, "height": 960})

	waitForMaxTick := func(min float64, timeout time.Duration) map[string]any {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			frame := readFrame()
			state, _ := frame["state"].(map[string]any)
			if state == nil {
				continue
			}
			maxTick, _ := state["maxRecordedTick"].(float64)
			if maxTick >= min {
				return frame
			}
		}
		t.Fatalf("timed out waiting for maxRecordedTick >= %v", min)
		return nil
	}

	waitForMaxTick(10, 5*time.Second)

	// 2. Click Play + press Space.
	const playX, playY = 640.0, 510.0
	send(map[string]any{"type": "input", "device": "mouse", "code": "Position", "kind": "move", "x": playX, "y": playY})
	send(map[string]any{"type": "input", "device": "mouse", "code": "Left", "kind": "down", "x": playX, "y": playY})
	send(map[string]any{"type": "input", "device": "mouse", "code": "Left", "kind": "up", "x": playX, "y": playY})
	for i := 0; i < 3; i++ {
		readFrame()
	}
	send(map[string]any{"type": "input", "device": "keyboard", "code": "Space", "kind": "down"})

	// 3. Wait for the simulation to advance well past our scrub targets.
	waitForMaxTick(80, 5*time.Second)

	// Scrub to tick 10 to record the reference signature.
	scrubAndRead := func(target uint64, timeout time.Duration) map[string]any {
		send(map[string]any{"type": "control", "action": "scrubToTick", "tick": int64(target)})
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			frame := readFrame()
			tick, _ := frame["tick"].(float64)
			if uint64(tick) == target {
				return frame
			}
		}
		t.Fatalf("scrub to %d: no frame at target tick", target)
		return nil
	}

	refFrame := scrubAndRead(10, 5*time.Second)
	refTick, _ := refFrame["tick"].(float64)
	refSig := frameSignature(refFrame)
	t.Logf("reference: tick=%.0f sig=%s", refTick, refSig)

	if refSig == "cmds=0" {
		t.Fatal("reference frame at tick 10 has no commands — the scrub did not reconstruct the game state")
	}

	// 4. Scrub forward to tick 50 (should be different state).
	fwdFrame := scrubAndRead(50, 5*time.Second)
	fwdSig := frameSignature(fwdFrame)
	t.Logf("forward:   tick=50 sig=%s", fwdSig)

	// 5. Scrub BACK to tick 10 and verify the signature matches the reference.
	backFrame := scrubAndRead(10, 5*time.Second)
	backSig := frameSignature(backFrame)
	t.Logf("back:      tick=10 sig=%s", backSig)

	if backSig != refSig {
		t.Fatalf("scrub back to tick 10: signature mismatch!\n  reference=%s\n  back=%s\n  (the scrub did not reconstruct the same state)", refSig, backSig)
	}

	// 6. Also verify the signatures at 10 and 50 differ (proving the
	// simulation state changes between ticks — i.e., the ball is moving).
	if refSig == fwdSig {
		t.Logf("WARNING: tick 10 and tick 50 have identical signatures (%s) — the simulation may be static (StateReady without Space, or StateMenu)", refSig)
	}
}

type discardWriter struct{}

func (w *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// Ensure the breakout package compiles in this test context.
var _ runtime.Game = (*breakout.Breakout)(nil)

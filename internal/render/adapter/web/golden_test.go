package web

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGoldenFrameDTO(t *testing.T) {
	frame := NewFrameDTOWithState(
		42,
		render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 150}),
			Aspect:   render.AspectFit,
		},
		render.RGBA(0.055, 0.065, 0.075, 1),
		StateDTO{
			Paused:           true,
			Speed:            0.5,
			Debug:            127,
			HistoryLen:       12,
			HistoryStartTick: 30,
			HistoryEndTick:   42,
			MaxRecordedTick:  48,
			RenderFPS:        60,
			SimulationHz:     120,
		},
		goldenCommands(),
	)

	got, err := json.MarshalIndent(frame, "", "  ")
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	got = append(got, '\n')

	assertGoldenBytes(t, "frame", got)
}

func assertGoldenBytes(t *testing.T, name string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name+".json")
	if *update {
		writeBytes(t, path, got)
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./internal/render/adapter/web -run TestGolden -update)", path, err)
	}

	if bytes.Equal(want, got) {
		return
	}

	failurePath := filepath.Join("testdata", "failures", name+".json")
	writeBytes(t, failurePath, got)
	t.Fatalf("%s differs from golden; got JSON saved to %s", name, failurePath)
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func goldenCommands() []render.Command {
	return []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Rect:        math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 48, Y: 28}),
			Fill:        render.RGBA(0.12, 0.62, 0.78, 0.55),
			Stroke:      render.RGBA(1, 1, 1, 0.42),
			StrokeWidth: 2,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Center:      math2d.Vec2{X: 52, Y: 32},
			Radius:      18,
			Fill:        render.RGBA(1, 0.36, 0.18, 0.45),
			Stroke:      render.RGBA(1, 0.82, 0.22, 1),
			StrokeWidth: 1.25,
		},
		render.DrawPoint{
			BaseCommand: render.BaseCommand{LayerValue: 4},
			Center:      math2d.Vec2{X: 22, Y: 54},
			Radius:      2.5,
			Color:       render.RGBA(0.4, 0.75, 1, 1),
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: 3},
			From:        math2d.Vec2{X: 8, Y: 63},
			To:          math2d.Vec2{X: 92, Y: 12},
			Width:       1.4,
			Color:       render.RGBA(0.82, 0.72, 1, 0.78),
		},
		render.DrawArc{
			BaseCommand: render.BaseCommand{LayerValue: 5},
			Center:      math2d.Vec2{X: 72, Y: 52},
			Radius:      13,
			StartAngle:  5.4,
			EndAngle:    0.7,
			Width:       2.2,
			Color:       render.RGBA(1, 0.4, 0.72, 0.82),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 6},
			Position:    math2d.Vec2{X: 50, Y: 20},
			Text:        "Golden",
			Color:       render.RGBA(0.95, 0.95, 0.95, 1),
			Size:        6.5,
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		},
	}
}

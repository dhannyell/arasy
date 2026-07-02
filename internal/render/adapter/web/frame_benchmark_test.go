package web

import (
	"encoding/json"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func BenchmarkFrameDTOMarshal(b *testing.B) {
	commands := make([]render.Command, 0, 500)
	for i := range 500 {
		x := float64(i % 128)
		y := float64(i / 128)
		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: x * 10, Y: y * 10},
				math2d.Vec2{X: 8, Y: 8},
			),
			Fill: render.RGBA(1, 0, 0, 0.8),
		})
	}

	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720}),
	}
	state := StateDTO{Speed: 1, HistoryLen: 300, SimulationHz: 60, RenderFPS: 60}

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		frame := NewFrameDTOWithState(uint64(i), camera, render.RGBA(0, 0, 0, 1), state, commands)
		if _, err := json.Marshal(frame); err != nil {
			b.Fatal(err)
		}
	}
}

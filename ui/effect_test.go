package ui

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestEffectComposesCommonTransforms(t *testing.T) {
	var store AnimStore
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 100}, math2d.Vec2{X: 20, Y: 5})

	effectAt := func(time float64) Effect {
		store.AdvanceFrame()
		u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 0.25, time)
		return u.Effect("fx", EffectOptions{Key: 1, Duration: 1})
	}

	effectAt(0)
	effect := effectAt(0.25)

	sliding := effect.SlideY(rect, -20, 100, 0, 1, ease.Linear)
	if sliding.Min.X != rect.Min.X || sliding.Min.Y != 10 || sliding.Size() != rect.Size() {
		t.Fatalf("SlideY rect = %+v, want x=%v y=10 size=%+v", sliding, rect.Min.X, rect.Size())
	}
	if alpha := effect.FadeOut(0, 1, ease.Linear); math.Abs(alpha-0.75) > 1e-9 {
		t.Fatalf("FadeOut = %v, want 0.75", alpha)
	}
	if pulse := effect.Pulse(0, 1, 0.1, 2*math.Pi); math.Abs(pulse-1.025) > 1e-9 {
		t.Fatalf("Pulse = %v, want 1.025", pulse)
	}
}

func TestFXToastEmitsCommandsUntilDone(t *testing.T) {
	var store AnimStore
	rect := math2d.NewRect(math2d.Vec2{X: 20, Y: 40}, math2d.Vec2{X: 30, Y: 10})
	opts := ToastOptions{
		Key:      1,
		Duration: 1,
		Layer:    77,
		Style: ToastStyle{
			Fill:        render.RGBA(0.2, 0.3, 0.4, 1),
			Border:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 2,
			TextColor:   render.RGBA(0.9, 0.8, 0.7, 1),
			TextSize:    12,
		},
	}

	store.AdvanceFrame()
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 0, 0)
	if !u.FX().Toast("toast", "Hi", rect, opts) {
		t.Fatal("Toast should render when just triggered")
	}
	commands := u.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	box, ok := commands[0].(render.DrawRect)
	if !ok {
		t.Fatalf("first command = %T, want DrawRect", commands[0])
	}
	if box.Layer() != 77 {
		t.Fatalf("box layer = %d, want 77", box.Layer())
	}
	if box.Rect.Min.Y != 20 {
		t.Fatalf("box Y = %v, want default slide-from Y 20", box.Rect.Min.Y)
	}
	text, ok := commands[1].(render.DrawText)
	if !ok {
		t.Fatalf("second command = %T, want DrawText", commands[1])
	}
	if text.Text != "Hi" || text.Layer() != 78 {
		t.Fatalf("text command = %+v, want text Hi on layer 78", text)
	}

	store.AdvanceFrame()
	u = New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, &store, 2, 2)
	if u.FX().Toast("toast", "Hi", rect, opts) {
		t.Fatal("Toast should stop rendering after duration")
	}
	if len(u.Commands()) != 0 {
		t.Fatalf("done toast emitted %d commands, want 0", len(u.Commands()))
	}
}

func TestToastDefaultMotionUnchanged(t *testing.T) {
	var store AnimStore
	rect := math2d.NewRect(math2d.Vec2{X: 20, Y: 40}, math2d.Vec2{X: 30, Y: 10})
	theme := DefaultTheme()

	toastCommandsAt := func(renderTime float64) (render.DrawRect, render.DrawText) {
		store.AdvanceFrame()
		u := New(&State{}, math2d.Vec2{}, true, testCam(), theme, testCam().Viewport, &store, 0.125, renderTime)
		if !u.FX().Toast("toast-default-motion", "Hi", rect, ToastOptions{Key: 1, Duration: 1}) {
			t.Fatalf("Toast at time %v should render", renderTime)
		}
		commands := u.Commands()
		if len(commands) != 2 {
			t.Fatalf("commands len at time %v = %d, want 2", renderTime, len(commands))
		}
		box, ok := commands[0].(render.DrawRect)
		if !ok {
			t.Fatalf("first command at time %v = %T, want DrawRect", renderTime, commands[0])
		}
		text, ok := commands[1].(render.DrawText)
		if !ok {
			t.Fatalf("second command at time %v = %T, want DrawText", renderTime, commands[1])
		}
		return box, text
	}

	toastCommandsAt(0)
	slideBox, slideText := toastCommandsAt(0.125)
	if math.Abs(slideBox.Rect.Min.Y-30) > 1e-9 {
		t.Fatalf("slide-in Y = %v, want 30", slideBox.Rect.Min.Y)
	}
	if slideBox.Fill.A != theme.Panel.Background.A {
		t.Fatalf("slide-in fill alpha = %v, want %v", slideBox.Fill.A, theme.Panel.Background.A)
	}
	if slideText.Size != theme.Label.Size {
		t.Fatalf("slide-in text size = %v, want %v", slideText.Size, theme.Label.Size)
	}

	fadeBox, fadeText := toastCommandsAt(0.875)
	if fadeBox.Rect.Min.Y != rect.Min.Y {
		t.Fatalf("fade-out Y = %v, want %v", fadeBox.Rect.Min.Y, rect.Min.Y)
	}
	if math.Abs(fadeBox.Fill.A-theme.Panel.Background.A*0.5) > 1e-9 {
		t.Fatalf("fade-out fill alpha = %v, want %v", fadeBox.Fill.A, theme.Panel.Background.A*0.5)
	}
	if fadeText.Size != theme.Label.Size {
		t.Fatalf("fade-out text size = %v, want %v", fadeText.Size, theme.Label.Size)
	}
}

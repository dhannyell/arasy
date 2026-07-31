package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func checkboxRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 30, Y: 30}, math2d.Vec2{X: 160, Y: 24})
}

func TestCheckboxClickToggles(t *testing.T) {
	state := &State{}
	next, changed := clickAt(state, checkboxRect().Center()).
		checkboxInputIn("injured", checkboxRect(), false, SpaceRef)

	if !next || !changed {
		t.Fatalf("CheckboxInput() = (%v, %v), want (true, true)", next, changed)
	}
}

func TestCheckboxUntouchedWithoutInput(t *testing.T) {
	cam := testInteractCamera()
	u := NewUpdate(&State{}, pointerFrame(), cam, DefaultTheme(), cam.Viewport)

	if next, changed := u.checkboxInputIn("injured", checkboxRect(), true, SpaceRef); !next || changed {
		t.Fatalf("CheckboxInput() = (%v, %v), want (true, false)", next, changed)
	}
}

// Activation from the keyboard is the same path as a click, so a focused
// checkbox answers Enter without the caller wiring anything.
func TestCheckboxKeyboardActivationToggles(t *testing.T) {
	state := &State{focusID: HashID("injured")}
	cam := testInteractCamera()
	u := NewUpdateWithInput(state, pointerFrame(), inputStateJustPressed(input.ActionConfirm), cam, DefaultTheme(), cam.Viewport)

	if next, changed := u.checkboxInputIn("injured", checkboxRect(), false, SpaceRef); !next || !changed {
		t.Fatalf("CheckboxInput() = (%v, %v), want (true, true)", next, changed)
	}
}

func TestCheckboxDrawsMarkOnlyWhenChecked(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	lines := func(checked bool) int {
		u := New(&State{}, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
		u.checkboxIn("injured", checkboxRect(), checked, "Injured", SpaceRef)
		count := 0
		for _, cmd := range u.Commands() {
			if _, ok := cmd.(render.DrawLine); ok {
				count++
			}
		}
		return count
	}

	if got := lines(false); got != 0 {
		t.Fatalf("unchecked drew %d mark strokes, want 0", got)
	}
	if got := lines(true); got != 2 {
		t.Fatalf("checked drew %d mark strokes, want 2", got)
	}
}

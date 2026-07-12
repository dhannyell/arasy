package ui

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func fontUI(t *testing.T, theme Theme) *UI {
	t.Helper()
	cam := testCamera()
	store := AnimStore{}
	return New(&State{}, math2d.Vec2{}, true, cam, theme, cam.Viewport, &store, 0, 0)
}

func lastDrawText(t *testing.T, cmds []render.Command) render.DrawText {
	t.Helper()
	for i := len(cmds) - 1; i >= 0; i-- {
		if dt, ok := cmds[i].(render.DrawText); ok {
			return dt
		}
	}
	t.Fatal("no DrawText command emitted")
	return render.DrawText{}
}

func TestThemedWidgetsUseThemeFont(t *testing.T) {
	theme := DefaultTheme()
	theme.Font = 7
	u := fontUI(t, theme)

	u.Label("hi", math2d.Vec2{X: 10, Y: 10})
	if got := lastDrawText(t, u.Commands()).Font; got != 7 {
		t.Fatalf("Label font = %d, want theme font 7", got)
	}
}

func TestZeroThemeKeepsBuiltinFont(t *testing.T) {
	u := fontUI(t, DefaultTheme()) // Font unset = built-in 0

	u.Label("hi", math2d.Vec2{X: 10, Y: 10})
	if got := lastDrawText(t, u.Commands()).Font; got != 0 {
		t.Fatalf("Label font = %d, want built-in font 0", got)
	}
}

func TestSetFontOverridesRawDrawTextOnly(t *testing.T) {
	theme := DefaultTheme()
	theme.Font = 7
	u := fontUI(t, theme)
	white := render.RGBA(1, 1, 1, 1)

	// Raw DrawText starts at the theme font each frame.
	u.DrawText("raw", math2d.Vec2{X: 1, Y: 1}, white, 10, render.AlignLeft, render.BaselineTop)
	if got := lastDrawText(t, u.Commands()).Font; got != 7 {
		t.Fatalf("raw DrawText font = %d, want theme font 7", got)
	}

	// SetFont changes it for subsequent raw draws.
	u.SetFont(3)
	u.DrawText("raw2", math2d.Vec2{X: 1, Y: 1}, white, 10, render.AlignLeft, render.BaselineTop)
	if got := lastDrawText(t, u.Commands()).Font; got != 3 {
		t.Fatalf("raw DrawText after SetFont = %d, want 3", got)
	}

	// Themed widgets ignore SetFont and keep the theme font.
	u.Label("hi", math2d.Vec2{X: 2, Y: 2})
	if got := lastDrawText(t, u.Commands()).Font; got != 7 {
		t.Fatalf("Label after SetFont = %d, want theme font 7 (unaffected)", got)
	}
}

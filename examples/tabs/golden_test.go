package tabs

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/replaytest"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

var update = flag.Bool("update", false, "rewrite golden fixtures")

// mouseMove, mouseDown, mouseUp build pointer events in screen-pixel space,
// matching what a browser host forwards.
func mouseMove(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove, X: x, Y: y,
	}
}

func mouseDown(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonDown, X: x, Y: y,
	}
}

func mouseUp(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonUp, X: x, Y: y,
	}
}

// headerCenter is the screen position of the i-th tab header. The camera maps
// world to screen one-to-one here, so world coordinates pass through.
func headerCenter(i int) math2d.Vec2 {
	r := tabsRect()
	width := r.Width() / float64(len(tabLabels))
	return math2d.Vec2{
		X: r.Min.X + (float64(i)+0.5)*width,
		Y: r.Min.Y + 16,
	}
}

func clickAt(d *runtime.Driver, pos math2d.Vec2) {
	d.ApplyInput([]input.Event{mouseMove(pos.X, pos.Y), mouseDown(pos.X, pos.Y), mouseUp(pos.X, pos.Y)})
	d.Step()
}

// TestGoldenReplay drives the demo through the live API — click each tab
// header, drag the slider that only the first tab owns — and pins the result
// to a committed ARRP recording plus a fingerprint of the final demo state.
// The demo holds simulation, so the checksum anchors alone say nothing here:
// the fingerprint carries the whole signal that tab selection reproduced.
func TestGoldenReplay(t *testing.T) {
	var last *Game
	newGame := func() runtime.Game {
		last = &Game{}
		return last
	}
	fingerprint := func(game runtime.Game) string {
		return fmt.Sprintf("%+v", game.(*Game).State)
	}

	script := func(d *runtime.Driver) {
		for _, tab := range []int{tabAssets, tabLog, tabScene, tabLog} {
			clickAt(d, headerCenter(tab))
			d.Step()
		}

		slider := densityRect(themedGeometry{barH: ui.DefaultTheme().Tabs.BarHeight})
		clickAt(d, headerCenter(tabScene))
		clickAt(d, math2d.Vec2{X: slider.Min.X + slider.Width()*0.75, Y: slider.Center().Y})

		if last.State.Switches == 0 {
			t.Fatal("golden replay script never switched tabs")
		}
	}

	replaytest.Golden(t, *update, "testdata", "tabs", newGame, fingerprint, script)
}

// themedGeometry answers TabsBodyRect the way a live ui.Drawer would, so the
// script can locate a widget inside the tab body without a UI instance. The
// demo leaves Config.UITheme unset, so the driver runs the default theme.
type themedGeometry struct{ barH float64 }

func (g themedGeometry) TabsBodyRect(r math2d.Rect) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: r.Min.X, Y: r.Min.Y + g.barH},
		math2d.Vec2{X: r.Width(), Y: r.Height() - g.barH},
	)
}

// TestClickingHeaderSwitchesTheRenderedBody is the end-to-end proof that the
// widget's two halves agree: a click processed in Update changes which body
// Render draws, on the live driver rather than on a hand-built ui.UI.
func TestClickingHeaderSwitchesTheRenderedBody(t *testing.T) {
	game := &Game{}
	driver := runtime.NewDriver(game, slog.New(slog.NewTextHandler(io.Discard, nil)))
	driver.Step()

	before := len(driver.Frame(0).Commands)
	if game.State.ActiveTab != tabScene {
		t.Fatalf("initial tab = %d, want %d", game.State.ActiveTab, tabScene)
	}

	clickAt(driver, headerCenter(tabLog))

	if game.State.ActiveTab != tabLog {
		t.Fatalf("tab after click = %d, want %d", game.State.ActiveTab, tabLog)
	}
	if game.State.Switches != 1 {
		t.Fatalf("switches = %d, want 1", game.State.Switches)
	}
	if after := len(driver.Frame(0).Commands); after == before {
		t.Fatalf("command count unchanged at %d; the body did not follow the active tab", after)
	}
}

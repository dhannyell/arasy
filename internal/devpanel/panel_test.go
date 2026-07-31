package devpanel

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

// testGame is the least a driver needs to run: a world nobody touches, so
// every assertion is about the panel and never about a simulation.
type testGame struct{}

func (testGame) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600}),
		},
	}
}

func (testGame) Init(*runtime.UpdateContext)                    {}
func (testGame) Update(*runtime.UpdateContext, float64)         {}
func (testGame) Render(*runtime.RenderContext) []render.Command { return nil }

func testViewport() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600})
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestPanel(t *testing.T, opts Options) *Panel {
	t.Helper()
	if opts.Log == nil {
		opts.Log = quietLog()
	}
	driver := runtime.NewDriver(testGame{}, opts.Log)
	driver.SkipSplash()
	driver.SetViewport(testViewport())
	return New(driver, opts)
}

// openPanel returns a visible panel already showing tab.
func openPanel(t *testing.T, opts Options, tab int) *Panel {
	t.Helper()
	p := newTestPanel(t, opts)
	p.SetVisible(true)
	p.activeTab = tab
	return p
}

func mouseEvent(kind input.EventKind, code input.SourceCode, pos math2d.Vec2) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: code},
		Kind:   kind,
		X:      pos.X,
		Y:      pos.Y,
	}
}

func moveTo(pos math2d.Vec2) input.Event {
	return mouseEvent(input.EventPointerMove, input.MouseX, pos)
}

func pressAt(pos math2d.Vec2) input.Event {
	return mouseEvent(input.EventButtonDown, input.MouseLeft, pos)
}

func releaseAt(pos math2d.Vec2) input.Event {
	return mouseEvent(input.EventButtonUp, input.MouseLeft, pos)
}

func keyEvent(kind input.EventKind, code input.SourceCode) input.Event {
	return input.Event{Source: input.Keyboard(code), Kind: kind}
}

// clickPanel drives one full click on a widget of the open panel, the way a
// host would: the pointer arrives, presses and releases across frames.
func clickPanel(p *Panel, pos math2d.Vec2) {
	p.HandleInput([]input.Event{moveTo(pos), pressAt(pos)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(pos)}, testViewport())
}

// center of the i-th button of the current tab's first row.
func rowButton(p *Panel, index, count int) math2d.Vec2 {
	lay := p.frameLayout(testViewport())
	return ui.Column(newRows(lay.body).next(), index, count, rowGap).Center()
}

func TestClosedPanelIsInert(t *testing.T) {
	p := newTestPanel(t, Options{})
	events := []input.Event{moveTo(math2d.Vec2{X: 700, Y: 40}), pressAt(math2d.Vec2{X: 700, Y: 40})}

	left := p.HandleInput(events, testViewport())
	if len(left) != len(events) {
		t.Fatalf("closed panel forwarded %d of %d events, want all of them", len(left), len(events))
	}
	if p.WantCaptureMouse() || p.WantCaptureKeyboard() {
		t.Fatal("closed panel captured input")
	}

	cmds := p.Render(frameCamera(), 16*time.Millisecond, Stats{})
	if cmds != nil {
		t.Fatalf("closed panel emitted %d commands, want none", len(cmds))
	}
}

func frameCamera() render.Camera {
	return render.Camera{World: testViewport(), Viewport: testViewport()}
}

func TestPauseButtonEmitsControlPause(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	if p.driver.State().Paused {
		t.Fatal("driver started paused; the test cannot tell the click apart")
	}

	clickPanel(p, rowButton(p, 0, 3))

	if !p.driver.State().Paused {
		t.Fatal("clicking Pause did not pause the driver")
	}
}

func TestPlayButtonResumesAPausedDriver(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	p.driver.Control(runtime.ControlCommand{Kind: runtime.ControlPause})

	clickPanel(p, rowButton(p, 0, 3))

	if p.driver.State().Paused {
		t.Fatal("clicking Play did not resume the driver")
	}
}

func TestSpeedSliderSetsDriverSpeed(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	lay := simRects(p.frameLayout(testViewport()).body)

	// Press at the far right of the track: the maximum of the interval.
	target := math2d.Vec2{X: lay.speed.Max.X - 1, Y: lay.speed.Center().Y}
	p.HandleInput([]input.Event{moveTo(target), pressAt(target)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(target)}, testViewport())

	if got := p.driver.State().Speed; got != speedRange.Max {
		t.Fatalf("driver speed = %v, want %v", got, speedRange.Max)
	}
}

// Hovering the panel is enough to take the mouse: the game must not react to
// a pointer that is over the tooling.
func TestHoverOverPanelCapturesTheMouse(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	lay := p.frameLayout(testViewport())
	inside := lay.panel.Center()

	left := p.HandleInput([]input.Event{moveTo(inside)}, testViewport())

	if !p.WantCaptureMouse() {
		t.Fatal("pointer over the panel did not capture the mouse")
	}
	if len(left) != 0 {
		t.Fatalf("forwarded %d events to the game, want none while hovered", len(left))
	}
}

func TestPointerOutsideThePanelIsForwarded(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	outside := math2d.Vec2{X: 40, Y: 500}

	left := p.HandleInput([]input.Event{moveTo(outside), pressAt(outside)}, testViewport())

	if p.WantCaptureMouse() {
		t.Fatal("pointer far from the panel captured the mouse")
	}
	if len(left) != 2 {
		t.Fatalf("forwarded %d events, want both", len(left))
	}
}

// The case a geometric hit test gets wrong: a slider dragged past the edge of
// the panel is still being dragged, and handing that motion to the game would
// steer the player mid-gesture.
func TestDragKeepsCaptureOutsideThePanel(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	lay := simRects(p.frameLayout(testViewport()).body)
	start := lay.speed.Center()

	p.HandleInput([]input.Event{moveTo(start), pressAt(start)}, testViewport())

	far := math2d.Vec2{X: 20, Y: 560}
	left := p.HandleInput([]input.Event{moveTo(far)}, testViewport())

	if !p.WantCaptureMouse() {
		t.Fatal("drag outside the panel lost the mouse")
	}
	if len(left) != 0 {
		t.Fatalf("forwarded %d events mid-drag, want none", len(left))
	}
}

// Clicking away from a focused field has to do two things at once: drop the
// focus, and still count as a click for the game underneath.
func TestClickOnEmptySpaceBlursAndPassesThrough(t *testing.T) {
	p := openPanel(t, Options{}, tabTimeline)
	lay := timelineRects(p.frameLayout(testViewport()).body)

	field := lay.tick.Center()
	p.HandleInput([]input.Event{moveTo(field), pressAt(field)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(field)}, testViewport())
	if !p.WantCaptureKeyboard() {
		t.Fatal("clicking the tick field did not focus it")
	}

	outside := math2d.Vec2{X: 40, Y: 500}
	left := p.HandleInput([]input.Event{moveTo(outside), pressAt(outside)}, testViewport())

	if p.WantCaptureKeyboard() {
		t.Fatal("clicking outside the panel kept the keyboard")
	}
	if len(left) != 2 {
		t.Fatalf("forwarded %d events, want the click to reach the game", len(left))
	}
}

// The keyboard only belongs to the panel while one of its widgets is being
// typed into; the rest of the time the game's keys are the game's.
func TestKeyboardPassesThroughWithoutFocus(t *testing.T) {
	p := openPanel(t, Options{}, tabTimeline)

	left := p.HandleInput([]input.Event{keyEvent(input.EventButtonDown, input.KeyArrowLeft)}, testViewport())

	if p.WantCaptureKeyboard() {
		t.Fatal("captured the keyboard with nothing focused")
	}
	if len(left) != 1 {
		t.Fatalf("forwarded %d key events, want 1", len(left))
	}
}

func TestFocusedFieldCapturesTypedText(t *testing.T) {
	p := openPanel(t, Options{}, tabTimeline)
	lay := timelineRects(p.frameLayout(testViewport()).body)
	field := lay.tick.Center()

	p.HandleInput([]input.Event{moveTo(field), pressAt(field)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(field)}, testViewport())

	text, ok := input.TextEvent('7')
	if !ok {
		t.Fatal("input.TextEvent rejected a digit")
	}
	left := p.HandleInput([]input.Event{text}, testViewport())

	if len(left) != 0 {
		t.Fatalf("forwarded %d events while typing, want none", len(left))
	}
	if p.tickText != "7" {
		t.Fatalf("tick field = %q, want %q", p.tickText, "7")
	}
}

func TestGoToTickSeeksTheDriver(t *testing.T) {
	p := openPanel(t, Options{}, tabTimeline)
	advance(p, 10)

	lay := timelineRects(p.frameLayout(testViewport()).body)
	field := lay.tick.Center()
	p.HandleInput([]input.Event{moveTo(field), pressAt(field)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(field)}, testViewport())
	text, _ := input.TextEvent('4')
	p.HandleInput([]input.Event{text}, testViewport())

	clickPanel(p, lay.goTick.Center())
	// The seek is consumed by the step that carries it, so the run resumes at
	// tick 4 and the same step lands on 5.
	p.driver.Step()

	if got := p.driver.State().Tick; got != 5 {
		t.Fatalf("tick after Go = %d, want 5 (seek to 4, then one step)", got)
	}
}

// Scrubbing seeks once, on release — not on every position the thumb passes
// through, which would reconstruct the world hundreds of times per drag.
func TestScrubSeeksOnReleaseOnly(t *testing.T) {
	p := openPanel(t, Options{}, tabTimeline)
	advance(p, 20)

	lay := timelineRects(p.frameLayout(testViewport()).body)
	start := lay.scrub.Center()

	p.HandleInput([]input.Event{moveTo(start), pressAt(start)}, testViewport())
	p.driver.Step()
	if got := p.driver.State().Tick; got != 21 {
		t.Fatalf("tick mid-drag = %d, want the run to have continued to 21", got)
	}

	p.HandleInput([]input.Event{releaseAt(start)}, testViewport())
	p.driver.Step()

	if got := p.driver.State().Tick; got > 20 {
		t.Fatalf("tick after release = %d, want a seek back into the recorded range", got)
	}
}

func TestLayersTabIsHiddenOutsideDevMode(t *testing.T) {
	plain := newTestPanel(t, Options{})
	if got := len(plain.tabs()); got != tabLayers {
		t.Fatalf("tabs without dev mode = %d, want %d", got, tabLayers)
	}

	dev := newTestPanel(t, Options{DevMode: true})
	if got := len(dev.tabs()); got != tabLayers+1 {
		t.Fatalf("tabs in dev mode = %d, want %d", got, tabLayers+1)
	}
}

func TestLayerCheckboxComposesTheMask(t *testing.T) {
	p := openPanel(t, Options{DevMode: true}, tabLayers)
	cells := layerRects(p.frameLayout(testViewport()).body)

	clickPanel(p, cells[0].Center())
	first := debugLayerSpec[0].flag
	if got := p.driver.State().DebugLayers; got != first {
		t.Fatalf("layers after one toggle = %v, want %v", got, first)
	}

	clickPanel(p, cells[1].Center())
	want := first | debugLayerSpec[1].flag
	if got := p.driver.State().DebugLayers; got != want {
		t.Fatalf("layers after two toggles = %v, want %v", got, want)
	}

	clickPanel(p, cells[0].Center())
	if got := p.driver.State().DebugLayers; got != debugLayerSpec[1].flag {
		t.Fatalf("layers after untoggling the first = %v, want %v", got, debugLayerSpec[1].flag)
	}
}

func TestTabSelectionFollowsHeaderClicks(t *testing.T) {
	p := openPanel(t, Options{DevMode: true}, tabSim)
	lay := p.frameLayout(testViewport())

	clickPanel(p, lay.headers[tabReplay].Center())

	if p.activeTab != tabReplay {
		t.Fatalf("active tab = %d, want %d", p.activeTab, tabReplay)
	}
}

func TestRenderStaysAboveGameLayers(t *testing.T) {
	p := openPanel(t, Options{DevMode: true}, tabSim)
	for _, tab := range []int{tabSim, tabTimeline, tabReplay, tabPerf, tabLayers} {
		p.activeTab = tab
		cmds := p.Render(frameCamera(), 16*time.Millisecond, Stats{Commands: 12})
		if len(cmds) == 0 {
			t.Fatalf("tab %d drew nothing", tab)
		}
		for _, cmd := range cmds {
			if cmd.Layer() < layerBase {
				t.Fatalf("tab %d emitted %T at layer %d, below the overlay band %d",
					tab, cmd, cmd.Layer(), layerBase)
			}
		}
	}
}

// Hiding the panel gives the input back immediately, mid-drag included.
func TestHidingReleasesCapture(t *testing.T) {
	p := openPanel(t, Options{}, tabSim)
	lay := simRects(p.frameLayout(testViewport()).body)
	start := lay.speed.Center()
	p.HandleInput([]input.Event{moveTo(start), pressAt(start)}, testViewport())

	p.SetVisible(false)

	left := p.HandleInput([]input.Event{moveTo(math2d.Vec2{X: 100, Y: 100})}, testViewport())
	if p.WantCaptureMouse() || len(left) != 1 {
		t.Fatal("hidden panel kept the mouse")
	}
}

// A panel authored at one size must keep its share of the window at any
// other: on a 4K screen a fixed-pixel card is a stamp in the corner.
func TestPanelKeepsItsShareOfTheWindow(t *testing.T) {
	p := newTestPanel(t, Options{})

	share := func(viewport math2d.Rect) (w, h float64) {
		lay := p.frameLayout(viewport)
		space := panelSpace(viewport)
		return lay.panel.Width() / space.Width(), lay.panel.Height() / space.Height()
	}

	hd := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080})
	uhd := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 3840, Y: 2160})

	hdW, hdH := share(hd)
	uhdW, uhdH := share(uhd)
	if hdW != uhdW || hdH != uhdH {
		t.Fatalf("panel share = (%.4f, %.4f) at 1080p and (%.4f, %.4f) at 4K, want the same",
			hdW, hdH, uhdW, uhdH)
	}
	if panelScale(uhd) <= panelScale(hd) {
		t.Fatalf("scale at 4K (%v) is not above 1080p (%v)", panelScale(uhd), panelScale(hd))
	}
}

// Below the authored size the panel holds still rather than shrinking: text
// that small is worse than a panel that takes more of a small window.
func TestPanelDoesNotShrinkBelowItsAuthoredSize(t *testing.T) {
	small := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600})
	if got := panelScale(small); got != 1 {
		t.Fatalf("scale on a small window = %v, want 1", got)
	}
}

// The panel space keeps the window's aspect ratio, so ui's fit resolves to a
// pure scale and the card still hugs the corner on an ultrawide.
func TestPanelHugsTheCornerOnAnUltrawide(t *testing.T) {
	p := newTestPanel(t, Options{})
	ultrawide := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 3440, Y: 1440})

	space := panelSpace(ultrawide)
	if got := space.Width() / space.Height(); got != ultrawide.Width()/ultrawide.Height() {
		t.Fatalf("panel space aspect = %v, want the window's %v", got, ultrawide.Width()/ultrawide.Height())
	}

	lay := p.frameLayout(ultrawide)
	if gap := space.Max.X - lay.panel.Max.X; gap != panelMargin {
		t.Fatalf("panel sits %v from the right edge, want the %v margin", gap, panelMargin)
	}
}

// Drawing and hit-testing have to agree under scaling, or the buttons stop
// being where they look.
func TestClicksLandOnScaledWidgets(t *testing.T) {
	p := newTestPanel(t, Options{})
	p.SetVisible(true)
	p.activeTab = tabSim

	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 3840, Y: 2160})
	scale := panelScale(viewport)
	lay := simRects(p.frameLayout(viewport).body)
	// Panel space to screen pixels: the click a user makes is on the pixel,
	// not on the unit.
	target := lay.buttons[0].Center().Mul(scale)

	p.HandleInput([]input.Event{moveTo(target), pressAt(target)}, viewport)
	p.HandleInput([]input.Event{releaseAt(target)}, viewport)

	if !p.driver.State().Paused {
		t.Fatal("clicking Pause on a 4K window missed the button")
	}
}

// advance runs the driver forward so the timeline has something to scrub.
func advance(p *Panel, ticks int) {
	for range ticks {
		p.driver.Step()
	}
}

// Determinism is the property replay and rewind rest on, so its failure gets
// the title bar — visible from every tab — rather than a stats line somebody
// has to go looking for.
func TestDivergedBadgeDrawsOnlyWhenViolated(t *testing.T) {
	p := newTestPanel(t, Options{})
	cam := frameCamera()

	badge := func(state runtime.DriveState) []render.Command {
		u := ui.New(&p.state, math2d.Vec2{}, false, cam, p.theme,
			panelSpace(cam.Viewport), &p.anim, 0, 0)
		p.drawDivergedBadge(u.Draw().In(ui.SpaceRef), p.frameLayout(cam.Viewport).panel, state)
		return raiseLayer(u.Commands())
	}

	if got := countFill(badge(runtime.DriveState{}), colorDanger); got != 0 {
		t.Fatalf("a healthy driver drew %d warning marks, want none", got)
	}

	violated := badge(runtime.DriveState{DeterminismViolated: true})
	if got := countFill(violated, colorDanger); got == 0 {
		t.Fatal("a diverged driver drew no warning")
	}
	if !containsText(violated, "DIVERGED") {
		t.Fatal("the warning has no label saying what is wrong")
	}
}

func containsText(commands []render.Command, want string) bool {
	for _, command := range commands {
		if text, ok := command.(render.DrawText); ok && text.Text == want {
			return true
		}
	}
	return false
}

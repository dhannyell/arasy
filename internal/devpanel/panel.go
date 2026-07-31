package devpanel

import (
	"log/slog"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

// Tab indices. Layers is last because it only exists in dev mode, so the
// three that are always there keep the same position either way.
const (
	tabSim = iota
	tabTimeline
	tabReplay
	tabLayers
)

var tabLabels = [...]string{"Sim", "Timeline", "Replay", "Layers"}

// Stats is the per-frame host telemetry the footer reports. The panel cannot
// measure either one: frame pacing is the host's clock, and the command count
// is what the host was handed for this frame.
type Stats struct {
	FPS      float64
	Commands int
}

// Options configures a panel. Every field is optional; the zero value gives a
// panel with the simulation controls and nothing that needs a host seam.
type Options struct {
	// DevMode gates the debug layer tab. Layer overlays cost a full extra
	// pass over the world, so they stay behind the same flag the host already
	// uses to decide whether this is a development build.
	DevMode bool

	// Recording is the host's file I/O for .arrp recordings. Nil disables
	// save and load; recording and watching still work in memory.
	Recording RecordingPort

	// NewGame builds a fresh game instance for verify and replay. Replay
	// mutates the game it is given, so each call needs a new one, and the
	// engine will not clone an arbitrary user type. Nil disables load.
	NewGame func() runtime.Game

	// Adopt is called when a loaded recording replaces the driver. The host
	// must repoint whatever it holds — the panel keeps driving the new one
	// either way, but the host would keep rendering the old.
	Adopt func(*runtime.Driver)

	// Theme overrides the panel styling. The zero value is the engine
	// default, filled in field by field like the game's own theme.
	Theme ui.Theme

	// Log receives panel diagnostics. Defaults to slog.Default().
	Log *slog.Logger
}

// Panel is the tooling overlay. It owns a ui.State of its own, deliberately
// separate from the game's: the panel is host chrome, so rewinding the
// simulation must not move its open tab, and its caret must not show up in a
// rewind snapshot.
type Panel struct {
	driver  *runtime.Driver
	adopt   func(*runtime.Driver)
	newGame func() runtime.Game
	log     *slog.Logger
	devMode bool

	state  ui.State
	theme  ui.Theme
	anim   ui.AnimStore
	router eventRouter

	visible   bool
	activeTab int
	time      float64

	// Caller-owned widget values. Every value widget in ui hands its value
	// back to whoever owns it; for the panel that is these fields, which is
	// also what lets a drag hold a value the driver has not accepted yet.
	speed       float64
	scrub       float64
	scrubActive bool
	tickText    string

	rec recorder

	wantMouse    bool
	wantKeyboard bool
}

// New mounts a panel over driver. It starts hidden: tooling is always
// available and never in the way until asked for.
func New(driver *runtime.Driver, opts Options) *Panel {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	p := &Panel{
		driver:  driver,
		adopt:   opts.Adopt,
		newGame: opts.NewGame,
		log:     log,
		devMode: opts.DevMode,
		theme:   ui.WithDefaults(opts.Theme),
		router:  newEventRouter(),
		speed:   1,
	}
	p.rec = recorder{port: opts.Recording, log: log}
	return p
}

// Driver returns the driver the panel currently drives, which is not the one
// it was built with once a recording has been loaded.
func (p *Panel) Driver() *runtime.Driver { return p.driver }

// Visible reports whether the panel is on screen.
func (p *Panel) Visible() bool { return p.visible }

// SetVisible shows or hides the panel. A hidden panel emits no commands and
// forwards every event untouched, so mounting one costs a host nothing until
// somebody opens it.
func (p *Panel) SetVisible(visible bool) {
	if p.visible == visible {
		return
	}
	p.visible = visible
	if !visible {
		// Dropping capture on the way out matters: a panel hidden mid-drag
		// would otherwise keep claiming the mouse it can no longer show.
		p.state = ui.State{}
		p.wantMouse = false
		p.wantKeyboard = false
		p.scrubActive = false
	}
}

// Toggle flips visibility, which is what the host's overlay key does.
func (p *Panel) Toggle() { p.SetVisible(!p.visible) }

// WantCaptureMouse reports whether the panel consumed this frame's pointer
// input, and WantCaptureKeyboard the same for the keyboard. Both are the
// answer for the last HandleInput call; hosts read them for diagnostics, the
// routing itself already happened.
func (p *Panel) WantCaptureMouse() bool    { return p.wantMouse }
func (p *Panel) WantCaptureKeyboard() bool { return p.wantKeyboard }

// HandleInput runs the panel's interaction pass over one batch of host events
// and returns the events the game should still receive.
//
// Every event reaches the widgets first, whether or not the pointer is over
// the panel — the panel has to see the click that lands outside it to know it
// lost focus, and the button release that ends a drag started inside. What it
// keeps is decided afterwards, from what the interaction actually did: the
// pointer is captured while the panel is hovered *or* a widget is active, and
// the keyboard only while a widget holds focus. Captured events are dropped
// here, so they never reach the driver and never enter a recording — clicking
// Pause is not part of the session being recorded.
func (p *Panel) HandleInput(events []input.Event, viewport math2d.Rect) []input.Event {
	if !p.visible || p.driver == nil {
		p.wantMouse = false
		p.wantKeyboard = false
		return events
	}

	frame, keys, actions := p.router.frame(events)
	cam := screenCamera(viewport)
	u := ui.NewUpdateWithKeys(&p.state, frame, keys, actions, cam, p.theme, viewport)
	in := u.Interact().In(ui.SpaceScreen)

	lay := p.layout(viewport, len(p.tabs()))
	p.update(in, lay)
	u.EndFrame()
	p.router.endFrame()

	p.wantMouse = in.Hovered(lay.panel) || p.state.ActiveID() != 0
	p.wantKeyboard = p.state.FocusID() != 0
	return p.forward(events)
}

// forward drops the events the panel captured and passes the rest along.
func (p *Panel) forward(events []input.Event) []input.Event {
	if !p.wantMouse && !p.wantKeyboard {
		return events
	}
	kept := make([]input.Event, 0, len(events))
	for _, event := range events {
		switch event.Source.Device {
		case input.DeviceMouse:
			if p.wantMouse {
				continue
			}
		case input.DeviceKeyboard:
			if p.wantKeyboard {
				continue
			}
		}
		kept = append(kept, event)
	}
	return kept
}

// Render returns the panel's commands for one frame, in the frame camera's
// world space and above every game layer. A closed panel returns nothing at
// all, which is what lets a host mount it unconditionally without moving a
// single pixel of a golden frame.
func (p *Panel) Render(cam render.Camera, elapsed time.Duration, stats Stats) []render.Command {
	if p.driver == nil {
		return nil
	}
	state := p.driver.State()
	// The deferred start of a recording is resolved every frame, open or
	// closed: hiding the panel must not strand a recording waiting for the
	// reset it asked for.
	p.rec.tick(p.driver, state)
	if !p.visible {
		return nil
	}

	dt := elapsed.Seconds()
	p.time += dt
	p.anim.AdvanceFrame()

	pointer := p.router.pointer
	u := ui.New(&p.state, pointer.Pos(), pointer.Inside(), cam, p.theme, cam.Viewport, &p.anim, dt, p.time)
	draw := u.Draw().In(ui.SpaceScreen)

	lay := p.layout(cam.Viewport, len(p.tabs()))
	p.draw(draw, lay, state, stats)
	return raiseLayer(u.Commands())
}

// tabs lists the tab labels for the current mode.
func (p *Panel) tabs() []string {
	if p.devMode {
		return tabLabels[:]
	}
	return tabLabels[:tabLayers]
}

func (p *Panel) update(in ui.InteractScope, lay layout) {
	state := p.driver.State()

	active, _ := in.TabsInputValue("devpanel#tabs", lay.headers, p.activeTab)
	p.activeTab = active

	switch active {
	case tabSim:
		p.updateSim(in, lay.body, state)
	case tabTimeline:
		p.updateTimeline(in, lay.body, state)
	case tabReplay:
		p.updateReplay(in, lay.body, state)
	case tabLayers:
		p.updateLayers(in, lay.body, state)
	}
}

func (p *Panel) draw(draw ui.DrawScope, lay layout, state runtime.DriveState, stats Stats) {
	draw.TitledPanel(lay.panel, "Arasy dev")
	draw.TabsAt(lay.headers, p.tabs(), p.activeTab)

	switch p.activeTab {
	case tabSim:
		p.drawSim(draw, lay.body, state)
	case tabTimeline:
		p.drawTimeline(draw, lay.body, state)
	case tabReplay:
		p.drawReplay(draw, lay.body, state)
	case tabLayers:
		p.drawLayers(draw, lay.body, state)
	}

	p.drawFooter(draw, lay.footer, state, stats)
}

func (p *Panel) drawFooter(draw ui.DrawScope, rect math2d.Rect, state runtime.DriveState, stats Stats) {
	maxTick := maxRecordedTick(state)
	mode := "live"
	if state.ReplayMode {
		mode = "watch"
	}

	lines := [2]string{
		"tick=" + formatUint(state.Tick) + "/" + formatUint(maxTick) +
			" paused=" + formatBool(state.Paused) +
			" speed=" + formatFloat(state.Speed) +
			" mode=" + mode,
		"fps=" + formatFloat(stats.FPS) +
			" cmds=" + formatInt(stats.Commands) +
			" rec=" + formatBool(p.rec.active()),
	}
	for i, line := range lines {
		p.text(draw, line, math2d.Vec2{
			X: rect.Min.X,
			Y: rect.Min.Y + rect.Height()*(float64(i)*2+1)/4,
		}, p.theme.Text.Muted, p.theme.Text.Caption)
	}
}

// text draws left-aligned body text, the shape almost every label in the
// panel wants and the one DrawScope.Label does not give (it centres).
func (p *Panel) text(draw ui.DrawScope, s string, pos math2d.Vec2, color render.Color, size float64) {
	draw.DrawText(s, pos, color, size, render.AlignLeft, render.BaselineMiddle)
}

func (p *Panel) control(cmd runtime.ControlCommand) {
	p.driver.Control(cmd)
}

// screenCamera is the identity camera for hit-testing: with World equal to
// Viewport, ui's screen space maps one-to-one onto host pixels, which is the
// space the panel lays itself out in.
func screenCamera(viewport math2d.Rect) render.Camera {
	return render.Camera{World: viewport, Viewport: viewport}
}

// maxRecordedTick is the far end of the scrub range. The live tick counts
// because a run that has never been rewound is still scrubbable up to now.
func maxRecordedTick(state runtime.DriveState) uint64 {
	if state.Tick > state.MaxRecordedTick {
		return state.Tick
	}
	return state.MaxRecordedTick
}

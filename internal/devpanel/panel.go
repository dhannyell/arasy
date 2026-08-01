//go:build arr_dev

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
// others keep the same position either way.
const (
	tabSim = iota
	tabTimeline
	tabReplay
	tabPerf
	tabInspect
	tabLayers
)

var tabLabels = [...]string{"Sim", "Timeline", "Replay", "Perf", "Inspect", "Layers"}

// Stats is the per-frame host telemetry the panel cannot measure for itself.
// Frame pacing is not here: the panel is handed the elapsed time of every
// frame anyway, so it keeps its own tracker rather than depending on a host
// having been asked for metrics.
//
// Every field below is optional. A host that measures none of them still
// gets a working Perf tab; it just shows less.
type Stats struct {
	// Commands is how many render commands the frame carried before the
	// panel appended its own.
	Commands int

	// CPUTick and CPURender split the host's frame between advancing the
	// simulation and building plus submitting the frame. They necessarily
	// describe the previous frame: this frame's render time is not known
	// until after the panel has been drawn into it.
	CPUTick   time.Duration
	CPURender time.Duration
	CPUSet    bool

	// InputLatency is host event arrival to render submission. It is not a
	// measurement of when the pixel reached the display.
	InputLatency    time.Duration
	InputLatencySet bool

	// HeapInUse and HeapTotal are the runtime's memory, which is what a
	// collector pause looks like before it becomes a frame spike.
	HeapInUse uint64
	HeapTotal uint64
	HeapSet   bool

	// Process and system memory come from OS probes only the native host can
	// run — a browser sandbox cannot see past its own JS heap. The group is
	// opt-in through MemSet: a host that cannot measure leaves the row out,
	// which is also the state every future non-Windows probe starts in.
	MemProcessBytes     uint64
	MemSystemUsedBytes  uint64
	MemSystemTotalBytes uint64
	MemSet              bool

	// VRAMProcessBytes is this process's video memory on the render adapter,
	// against the budget the OS grants it. Same opt-in discipline as MemSet.
	VRAMProcessBytes uint64
	VRAMBudgetBytes  uint64
	VRAMSet          bool

	// GPU utilization, 0-100: this process's engines versus every engine on
	// the system, so a saturated GPU caused by *another* process reads
	// differently from one the game itself is causing.
	GPUProcessPercent float64
	GPUSystemPercent  float64
	GPUSet            bool

	// GPU execution time per pass, from timestamp queries: the particle emit
	// and simulate compute passes and the render interval. This is what the
	// GPU actually spent, not the utilization percentage above and not the
	// CPU's view of submission. The sample lags its frame, because reading
	// timestamps back without stalling the pipeline means reading them late.
	GPUEmitTime     time.Duration
	GPUSimulateTime time.Duration
	GPURenderTime   time.Duration
	GPUTimeSet      bool

	// Garbage collection since the previous frame. GCCycles above zero marks
	// the frame in the graph, which is the whole point: it turns "it stutters
	// sometimes" into a bar you can point at.
	//
	// GCAllocBytes is deliberately the only attribution offered. No function
	// forces a collection — a cycle starts when the heap reaches its goal, and
	// the allocation that crosses the line is arbitrary — so naming a culprit
	// call site would be naming the wrong one most of the time. The allocation
	// rate is what actually sets how often the collector runs.
	GCCycles     uint64
	GCPause      time.Duration
	GCAllocBytes uint64
	GCHeapLive   uint64
	GCHeapGoal   uint64
	GCSet        bool

	// PresentWait is how long the last frame blocked on the presentation
	// queue (acquire + present). With vsync that is the vblank wait — idle
	// time that must not be read as CPU render work, so hosts that measure
	// it report it apart.
	PresentWait time.Duration
	PresentSet  bool
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

	// The panel measures frame pacing itself, from the elapsed time every
	// host already hands it. Depending on the host's own tracker would mean
	// the Perf tab worked only for apps that had asked for metrics.
	frames     runtime.FrameStatsTracker
	frameStats runtime.FrameStats
	history    frameHistory
	stats      Stats

	// Caller-owned widget values. Every value widget in ui hands its value
	// back to whoever owns it; for the panel that is these fields, which is
	// also what lets a drag hold a value the driver has not accepted yet.
	speed       float64
	scrub       float64
	scrubActive bool
	tickText    string

	inspect   inspectState
	pickArmed bool
	// pickAwaitRelease consumes the button-up paired with a consumed pick
	// click: forwarding a release whose press the game never saw would be
	// worse than dropping both.
	pickAwaitRelease bool
	// frameCam is the camera the last Render was handed, cached so the pick
	// gesture can map a screen click back into game world space.
	frameCam render.Camera

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
	// The max uint forces the first inspector refresh: tick 0 is a real tick,
	// so "same as last refresh" cannot be the zero value.
	p.inspect.tick = ^uint64(0)
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
		p.pickArmed = false
		p.inspect.edit = inspectEdit{}
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
	u := ui.NewUpdateWithKeys(&p.state, frame, keys, actions, cam, p.theme, panelSpace(viewport))
	in := u.Interact().In(ui.SpaceRef)

	lay := p.frameLayout(viewport)
	p.update(in, lay)
	u.EndFrame()
	p.router.endFrame()

	p.wantMouse = in.Hovered(lay.panel) || p.state.ActiveID() != 0
	p.wantKeyboard = p.state.FocusID() != 0
	if p.handlePick(events, viewport, lay) {
		// The pick click landed on the world, not on a widget, so nothing
		// above claimed it — claim it here, or the game would also get the
		// click that was meant as an inspection.
		p.wantMouse = true
	}
	return p.forward(events)
}

// handlePick resolves the pick gesture while it is armed: a left click
// outside the panel selects the topmost entity under it (in game world
// space, through the frame camera), a right click cancels. Escape only
// disarms — it still goes to the game, since the keyboard is the game's
// unless a widget holds focus. Returns whether the gesture consumed the
// pointer this frame.
func (p *Panel) handlePick(events []input.Event, viewport math2d.Rect, lay layout) bool {
	consumed := false
	if p.pickAwaitRelease {
		for _, event := range events {
			if event.Source.Code == input.MouseLeft && event.Kind == input.EventButtonUp {
				p.pickAwaitRelease = false
				consumed = true
			}
		}
	}
	if !p.pickArmed {
		return consumed
	}
	for _, event := range events {
		switch event.Source.Code {
		case input.MouseLeft:
			if event.Kind != input.EventButtonDown {
				continue
			}
			pos := math2d.Vec2{X: event.X, Y: event.Y}
			if p.panelContainsScreen(viewport, lay, pos) {
				continue // clicks on the panel itself stay widget business
			}
			worldPos := p.frameCam.ScreenToWorld(pos)
			if id, ok := pickEntity(p.driver.WorldView(), p.inspect.entities, worldPos); ok {
				p.inspect.mode = inspectModeWorld
				p.inspect.entity = id
				p.inspect.hasEntity = true
				p.inspect.edit = inspectEdit{}
			}
			p.pickArmed = false
			p.pickAwaitRelease = true
			consumed = true
		case input.MouseRight:
			if event.Kind == input.EventButtonDown {
				p.pickArmed = false
				consumed = true
			}
		case input.KeyEscape:
			if event.Kind == input.EventButtonDown {
				p.pickArmed = false
			}
		}
	}
	return consumed
}

// panelContainsScreen tests a screen-space point against the panel rect,
// which is laid out in panel units — the same conversion the ui does for
// its own hit-tests.
func (p *Panel) panelContainsScreen(viewport math2d.Rect, lay layout, pos math2d.Vec2) bool {
	scale := panelScale(viewport)
	return lay.panel.Contains(math2d.Vec2{X: pos.X / scale, Y: pos.Y / scale})
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
	p.frameCam = cam
	state := p.driver.State()
	// The deferred start of a recording is resolved every frame, open or
	// closed: hiding the panel must not strand a recording waiting for the
	// reset it asked for.
	p.rec.tick(p.driver, state)
	// Frame pacing is sampled while closed too. A history that only began
	// when the panel opened would be blank exactly when somebody opens it to
	// find out what just happened.
	p.observeFrame(elapsed, stats)
	if !p.visible {
		return nil
	}

	dt := elapsed.Seconds()
	p.time += dt
	p.anim.AdvanceFrame()

	pointer := p.router.pointer
	u := ui.New(&p.state, pointer.Pos(), pointer.Inside(), cam, p.theme, panelSpace(cam.Viewport), &p.anim, dt, p.time)
	draw := u.Draw().In(ui.SpaceRef)

	lay := p.frameLayout(cam.Viewport)
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
	p.enterTab(active)

	switch active {
	case tabSim:
		p.updateSim(in, lay.body, state)
	case tabTimeline:
		p.updateTimeline(in, lay.body, state)
	case tabReplay:
		p.updateReplay(in, lay.body, state)
	case tabInspect:
		p.updateInspect(in, lay.body, state)
	case tabLayers:
		p.updateLayers(in, lay.body, state)
	}
	// The Perf tab is a readout: it has nothing to interact with.
}

// enterTab switches tabs, doing the work a tab needs done once on arrival
// rather than every frame it is open. Listing saved recordings reads the
// filesystem, and the set of files only changes when this panel changes it.
func (p *Panel) enterTab(tab int) {
	if tab == p.activeTab {
		return
	}
	p.activeTab = tab
	if tab == tabReplay {
		p.rec.refresh()
	}
}

// observeFrame folds one frame into the pacing history.
func (p *Panel) observeFrame(elapsed time.Duration, stats Stats) {
	p.stats = stats
	if frame, ready := p.frames.Observe(elapsed); ready {
		p.frameStats = frame
	}
	p.history.push(elapsed, stats.GCSet && stats.GCCycles > 0)
}

func (p *Panel) draw(draw ui.DrawScope, lay layout, state runtime.DriveState, stats Stats) {
	// Pick targets come first: the panel chrome is drawn over them, so an
	// entity under the panel stays hidden instead of glowing through.
	if p.pickArmed {
		p.drawPickTargets(draw)
	}
	draw.TitledPanel(lay.panel, "Arasy dev")
	p.drawDivergedBadge(draw, lay.panel, state)
	draw.TabsAt(lay.headers, p.tabs(), p.activeTab)

	switch p.activeTab {
	case tabSim:
		p.drawSim(draw, lay.body, state)
	case tabTimeline:
		p.drawTimeline(draw, lay.body, state)
	case tabReplay:
		p.drawReplay(draw, lay.body, state)
	case tabPerf:
		p.drawPerf(draw, lay.body)
	case tabInspect:
		p.drawInspect(draw, lay.body, state)
	case tabLayers:
		p.drawLayers(draw, lay.body, state)
	}

	p.drawFooter(draw, lay.footer, state, stats)
}

// drawDivergedBadge marks a driver that reconstructed a tick and got a
// different world than the one it recorded. Determinism is the property the
// whole replay and rewind machinery rests on, so its failure belongs in the
// title bar, visible from every tab, rather than in a stats line somebody has
// to go looking for.
func (p *Panel) drawDivergedBadge(draw ui.DrawScope, panel math2d.Rect, state runtime.DriveState) {
	if !state.DeterminismViolated {
		return
	}

	header := math2d.NewRect(panel.Min, math2d.Vec2{X: panel.Width(), Y: p.theme.Header.Height})
	badge, _ := ui.CutRight(ui.Inset(header, badgeInset), badgeWidth)

	draw.FillRect(badge, colorDanger)
	draw.DrawText("DIVERGED", badge.Center(), p.theme.Text.Color, p.theme.Text.Caption,
		render.AlignCenter, render.BaselineMiddle)
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
		"fps=" + formatFloat(p.frameStats.FPS) +
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

// drawPickTargets outlines every pickable entity in the game world: while
// the pick gesture is armed, the user should not have to guess which things
// on screen are world components and which are backdrop or cosmetics. The
// outline is the collider — the same shape the click will see.
func (p *Panel) drawPickTargets(draw ui.DrawScope) {
	cam := p.frameCam
	if cam.Viewport.Width() <= 0 || cam.Viewport.Height() <= 0 {
		return
	}
	view := p.driver.WorldView()
	scale := panelScale(cam.Viewport)

	// The panel draws in panel units; the pick shapes live in game world
	// space. screenPerWorld converts lengths, measured from the camera's
	// own mapping so aspect resolution cannot disagree with it.
	origin := cam.WorldToScreen(math2d.Vec2{})
	unit := cam.WorldToScreen(math2d.Vec2{X: 1})
	screenPerWorld := unit.X - origin.X

	for _, id := range p.inspect.entities {
		transform, ok := view.Transform(id)
		if !ok {
			continue
		}
		center := cam.WorldToScreen(transform.Position)
		center = math2d.Vec2{X: center.X / scale, Y: center.Y / scale}

		if circle, ok := view.CircleCollider(id); ok {
			draw.DrawCircle(center, circle.Radius*screenPerWorld/scale,
				render.Color{}, colorAction, 1.5)
			continue
		}
		if rect, ok := view.RectCollider(id); ok {
			hx := rect.HalfExtends.X * screenPerWorld / scale
			hy := rect.HalfExtends.Y * screenPerWorld / scale
			draw.DrawPoly(render.Color{}, colorAction, 1.5,
				math2d.Vec2{X: center.X - hx, Y: center.Y - hy},
				math2d.Vec2{X: center.X + hx, Y: center.Y - hy},
				math2d.Vec2{X: center.X + hx, Y: center.Y + hy},
				math2d.Vec2{X: center.X - hx, Y: center.Y + hy},
			)
		}
	}
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

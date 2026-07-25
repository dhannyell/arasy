//go:build js && wasm

package wasm

import (
	"strconv"
	"syscall/js"

	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// debugPanel is the in-browser dev overlay for the WASM host. It drives the
// runtime driver directly through the same public control surface used by the
// canvas dev server.
type debugPanel struct {
	host *host

	root    js.Value
	visible bool
	devMode bool

	statsLabel js.Value
	speed      js.Value
	speedLabel js.Value
	scrub      js.Value
	scrubLabel js.Value
	tickInput  js.Value
	playPause  js.Value
	checks     map[scene.DebugLayers]js.Value

	recorder       *recordingPanel
	recordingLabel js.Value
	recordingTools js.Value

	callbacks    []js.Func
	scrubbing    bool
	speedEditing bool
}

var debugLayerSpec = []struct {
	label string
	flag  scene.DebugLayers
}{
	{"Colliders", scene.DebugColliders},
	{"Contacts", scene.DebugContacts},
	{"Grid", scene.DebugGrid},
	{"Active grid", scene.DebugActiveGrid},
	{"Candidate grid", scene.DebugCandidateGrid},
	{"Velocity", scene.DebugVelocities},
	{"Entity ID", scene.DebugEntityIDs},
	{"Render layers", scene.DebugRenderLayers},
	{"Input state", scene.DebugInputState},
	{"Runtime stats", scene.DebugRuntimeStats},
}

func newDebugPanel(h *host, devMode bool, newGame func() runtime.Game) *debugPanel {
	p := &debugPanel{host: h, devMode: devMode, checks: make(map[scene.DebugLayers]js.Value)}
	doc := js.Global().Get("document")

	style := doc.Call("createElement", "style")
	style.Set("textContent", debugPanelCSS)
	doc.Get("head").Call("appendChild", style)

	root := doc.Call("createElement", "div")
	root.Set("id", "ff-debug-panel")
	root.Get("style").Set("display", "none")
	root.Set("innerHTML", debugPanelHTML)
	doc.Get("body").Call("appendChild", root)

	p.root = root
	p.statsLabel = doc.Call("getElementById", "ff-debug-stats")
	p.speed = doc.Call("getElementById", "ff-debug-speed")
	p.speedLabel = doc.Call("getElementById", "ff-debug-speed-label")
	p.scrub = doc.Call("getElementById", "ff-debug-scrub")
	p.scrubLabel = doc.Call("getElementById", "ff-debug-scrub-label")
	p.tickInput = doc.Call("getElementById", "ff-debug-tick")
	p.playPause = doc.Call("getElementById", "ff-debug-play-pause")
	p.recordingLabel = doc.Call("getElementById", "ff-debug-recording")
	p.recordingTools = doc.Call("getElementById", "ff-recording-tools")

	p.retainClick(p.playPause, p.togglePlayPause)
	p.retainClick(doc.Call("getElementById", "ff-debug-step"), func() {
		p.control(runtime.ControlCommand{Kind: runtime.ControlStep})
	})
	p.retainClick(doc.Call("getElementById", "ff-debug-back"), func() {
		p.control(runtime.ControlCommand{Kind: runtime.ControlBack})
	})
	p.retainClick(doc.Call("getElementById", "ff-debug-back-second"), func() {
		p.control(runtime.ControlCommand{Kind: runtime.ControlBackSeconds, Seconds: 1})
	})
	p.retainClick(doc.Call("getElementById", "ff-debug-reset"), func() {
		p.control(runtime.ControlCommand{Kind: runtime.ControlReset})
	})
	p.retainClick(doc.Call("getElementById", "ff-debug-go-tick"), p.goToTick)
	p.retainEvent(p.speed, "pointerdown", func() { p.speedEditing = true })
	p.retainEvent(p.speed, "pointerup", func() { p.speedEditing = false })
	p.retainEvent(p.speed, "blur", func() { p.speedEditing = false })
	p.retainInput(p.speed, p.applySpeed)
	p.retainChange(p.speed, func() {
		p.applySpeed()
		p.speedEditing = false
	})
	p.retainInput(p.scrub, p.previewScrub)
	p.retainChange(p.scrub, p.applyScrub)

	if devMode {
		p.buildDebugLayerControls(doc)
	} else {
		doc.Call("getElementById", "ff-debug-controls").Get("style").Set("display", "none")
	}

	if newGame != nil {
		p.recordingTools.Get("style").Set("display", "block")
		p.recorder = newRecordingPanel(h, newGame, h.log)
	} else {
		p.recordingTools.Get("style").Set("display", "none")
	}

	return p
}

func (p *debugPanel) buildDebugLayerControls(doc js.Value) {
	layers := doc.Call("getElementById", "ff-debug-layers")
	for i, spec := range debugLayerSpec {
		flag := spec.flag
		id := "ff-debug-layer-" + strconv.Itoa(i)
		label := doc.Call("createElement", "label")
		label.Set("className", "ff-debug-check")
		input := doc.Call("createElement", "input")
		input.Set("id", id)
		input.Set("type", "checkbox")
		text := doc.Call("createTextNode", spec.label)
		label.Call("appendChild", input)
		label.Call("appendChild", text)
		layers.Call("appendChild", label)
		p.checks[flag] = input
		p.retainChange(input, func() {
			p.control(runtime.ControlCommand{
				Kind:        runtime.ControlSetDebugLayers,
				DebugLayers: p.debugMask(),
			})
		})
	}
}

func (p *debugPanel) setVisible(visible bool) {
	p.visible = visible
	if visible {
		p.root.Get("style").Set("display", "block")
		return
	}
	p.root.Get("style").Set("display", "none")
}

func (p *debugPanel) control(cmd runtime.ControlCommand) {
	p.host.driver.Control(cmd)
}

func (p *debugPanel) retainClick(el js.Value, fn func()) {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn()
		return nil
	})
	p.callbacks = append(p.callbacks, cb)
	el.Call("addEventListener", "click", cb)
}

func (p *debugPanel) retainInput(el js.Value, fn func()) {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn()
		return nil
	})
	p.callbacks = append(p.callbacks, cb)
	el.Call("addEventListener", "input", cb)
}

func (p *debugPanel) retainChange(el js.Value, fn func()) {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn()
		return nil
	})
	p.callbacks = append(p.callbacks, cb)
	el.Call("addEventListener", "change", cb)
}

func (p *debugPanel) retainEvent(el js.Value, event string, fn func()) {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn()
		return nil
	})
	p.callbacks = append(p.callbacks, cb)
	el.Call("addEventListener", event, cb)
}

func (p *debugPanel) togglePlayPause() {
	state := p.host.driver.State()
	if state.Paused {
		p.control(runtime.ControlCommand{Kind: runtime.ControlPlay})
		return
	}
	p.control(runtime.ControlCommand{Kind: runtime.ControlPause})
}

func (p *debugPanel) goToTick() {
	tick, ok := parseUintValue(p.tickInput)
	if !ok {
		return
	}
	p.control(runtime.ControlCommand{Kind: runtime.ControlBackToTick, Tick: tick})
}

func (p *debugPanel) applySpeed() {
	p.speedEditing = true
	speed := parseFloatValue(p.speed, 1)
	p.speedLabel.Set("textContent", formatFloat(speed))
	p.control(runtime.ControlCommand{
		Kind:  runtime.ControlSetSpeed,
		Speed: speed,
	})
}

func (p *debugPanel) previewScrub() {
	p.scrubbing = true
	p.scrubLabel.Set("textContent", p.scrub.Get("value").String())
}

func (p *debugPanel) applyScrub() {
	p.scrubbing = false
	tick, ok := parseUintValue(p.scrub)
	if !ok {
		return
	}
	p.control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: tick})
}

func (p *debugPanel) debugMask() scene.DebugLayers {
	var mask scene.DebugLayers
	for _, spec := range debugLayerSpec {
		if p.checks[spec.flag].Get("checked").Bool() {
			mask |= spec.flag
		}
	}
	return mask
}

func (p *debugPanel) sync(state runtime.DriveState, fps int, cmdCount int) {
	if p.root.IsNull() {
		return
	}

	p.statsLabel.Set("textContent",
		"tick="+formatUint(state.Tick)+
			" paused="+boolStr(state.Paused)+
			" speed="+formatFloat(state.Speed)+
			" rec="+boolStr(p.recording())+
			" fps="+itoa(fps)+
			" cmds="+itoa(cmdCount))
	p.recordingLabel.Set("textContent", "recording="+boolStr(p.recording()))

	if state.Paused {
		p.playPause.Set("textContent", "Play")
	} else {
		p.playPause.Set("textContent", "Pause")
	}

	if !p.speedEditing {
		p.speed.Set("value", formatFloat(state.Speed))
		p.speedLabel.Set("textContent", formatFloat(state.Speed))
	}

	maxTick := state.MaxRecordedTick
	if state.Tick > maxTick {
		maxTick = state.Tick
	}
	p.scrub.Set("min", "0")
	p.scrub.Set("max", formatUint(maxTick))
	p.tickInput.Set("max", formatUint(maxTick))
	if !p.scrubbing {
		p.scrub.Set("value", formatUint(state.Tick))
		p.scrubLabel.Set("textContent", formatUint(state.Tick)+"/"+formatUint(maxTick))
	}

	if p.devMode {
		for _, spec := range debugLayerSpec {
			p.checks[spec.flag].Set("checked", state.DebugLayers.Has(spec.flag))
		}
	}
	if p.recorder != nil {
		p.recorder.sync(state)
	}
}

func (p *debugPanel) recording() bool {
	return p.recorder != nil && p.recorder.recording
}

func parseUintValue(el js.Value) (uint64, bool) {
	v, err := strconv.ParseUint(el.Get("value").String(), 10, 64)
	return v, err == nil
}

func parseFloatValue(el js.Value, fallback float64) float64 {
	v, err := strconv.ParseFloat(el.Get("value").String(), 64)
	if err != nil {
		return fallback
	}
	return v
}

func formatUint(v uint64) string {
	return strconv.FormatUint(v, 10)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

const debugPanelHTML = `
<div class="ff-debug-title">frameforge debug</div>
<div id="ff-debug-stats" class="ff-debug-stats">tick=0</div>
<div class="ff-debug-row ff-debug-status-row">
  <span id="ff-debug-recording">recording=false</span>
</div>
<div id="ff-debug-controls">
  <div class="ff-debug-row">
    <button id="ff-debug-play-pause" type="button">Pause</button>
    <button id="ff-debug-step" type="button">+1 tick</button>
    <button id="ff-debug-back" type="button">Back</button>
    <button id="ff-debug-back-second" type="button">Back 1s</button>
    <button id="ff-debug-reset" type="button">Reset</button>
  </div>
  <label class="ff-debug-field">speed
    <input id="ff-debug-speed" type="range" min="0.25" max="8" step="0.25" value="1">
    <span id="ff-debug-speed-label">1.00</span>
  </label>
  <label class="ff-debug-field">scrub
    <input id="ff-debug-scrub" type="range" min="0" max="0" step="1" value="0">
    <span id="ff-debug-scrub-label">0/0</span>
  </label>
  <div class="ff-debug-row">
    <input id="ff-debug-tick" type="number" min="0" step="1" value="0" aria-label="tick">
    <button id="ff-debug-go-tick" type="button">Go tick</button>
  </div>
  <details open>
    <summary>layers</summary>
    <div id="ff-debug-layers" class="ff-debug-layers"></div>
  </details>
</div>
<div id="ff-recording-tools" class="ff-debug-section">
  <div class="ff-debug-row">
    <button id="ff-rec-record" type="button">Record</button>
    <button id="ff-rec-save" type="button">Save .ffrp</button>
    <button id="ff-rec-load" type="button">Load .ffrp</button>
    <input id="ff-rec-file" type="file" accept=".ffrp" style="display:none">
  </div>
  <div id="ff-rec-status" class="ff-debug-message">recording idle</div>
</div>
`

const debugPanelCSS = `
#ff-debug-panel {
	position: fixed;
	top: 8px;
	right: 8px;
	z-index: 1000;
	width: min(380px, calc(100vw - 16px));
	box-sizing: border-box;
	padding: 10px 12px;
	border: 1px solid rgba(255,255,255,0.16);
	border-radius: 6px;
	background: rgba(12,16,20,0.88);
	color: #e8edf2;
	font: 12px/1.4 ui-monospace, SFMono-Regular, Consolas, monospace;
	box-shadow: 0 10px 30px rgba(0,0,0,0.32);
	user-select: none;
}
#ff-debug-panel button,
#ff-debug-panel input {
	font: inherit;
}
#ff-debug-panel button {
	background: #202832;
	color: #f4f7fa;
	border: 1px solid rgba(255,255,255,0.18);
	border-radius: 4px;
	padding: 3px 7px;
}
#ff-debug-panel input[type="number"] {
	width: 104px;
	box-sizing: border-box;
	background: #10161d;
	color: #f4f7fa;
	border: 1px solid rgba(255,255,255,0.18);
	border-radius: 4px;
	padding: 3px 6px;
}
.ff-debug-title {
	font-weight: 700;
	margin-bottom: 4px;
}
.ff-debug-stats {
	opacity: 0.82;
	margin-bottom: 8px;
	white-space: normal;
	overflow-wrap: anywhere;
}
.ff-debug-row {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
	align-items: center;
	margin-bottom: 8px;
}
.ff-debug-status-row {
	margin-bottom: 6px;
	opacity: 0.82;
}
.ff-debug-field {
	display: grid;
	grid-template-columns: 44px 1fr 44px;
	gap: 8px;
	align-items: center;
	margin-bottom: 8px;
}
.ff-debug-layers {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 4px 10px;
	margin-top: 6px;
}
.ff-debug-check {
	display: flex;
	gap: 5px;
	align-items: center;
	white-space: nowrap;
}
.ff-debug-section {
	border-top: 1px solid rgba(255,255,255,0.12);
	margin-top: 10px;
	padding-top: 8px;
}
.ff-debug-message {
	opacity: 0.78;
	overflow-wrap: anywhere;
}
`

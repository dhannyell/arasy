package canvas

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/render/adapter/web"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

//go:embed static
var staticFiles embed.FS

type Options struct {
	Addr string
	Log  *slog.Logger
}

func Run(game runtime.Game, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	driver := runtime.NewDriver(game, log)
	cfg := driver.Config()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := &hub{
		driver:  driver,
		fps:     cfg.FPS,
		simHz:   cfg.SimulationHz,
		clear:   cfg.Clear,
		camera:  cfg.Camera,
		clients: make(map[*client]struct{}),
	}
	go h.run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/ws", h.handleWS)
	mux.Handle("/static/", http.FileServerFS(staticFiles))

	srv := &http.Server{
		Addr:              opts.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Info("canvas listening", "addr", opts.Addr)
	return srv.ListenAndServe()
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, staticFiles, "static/index.html")
}

// ---------------------------------------------------------------------------
// hub — transport only. No game logic, no control logic.
// ---------------------------------------------------------------------------

type hub struct {
	driver *runtime.Driver
	fps    int
	simHz  int
	clear  render.Color
	camera render.Camera

	// inbox: escrito pelos handlers WS, drenado pela simLoop no início de cada tick
	inboxMu       sync.Mutex
	pendingEvents []input.Event
	pendingCmds   []runtime.ControlCommand

	// worldMu: Advance exclusivo, Commands compartilhado
	worldMu sync.RWMutex

	clientsMu sync.RWMutex
	clients   map[*client]struct{}
}

type client struct {
	conn   *websocket.Conn
	mu     sync.RWMutex
	camera render.Camera
}

// clientMessage is the JSON envelope coming from the browser. Web-specific knowledge.
type clientMessage struct {
	Type   string  `json:"type"`
	Action string  `json:"action"`
	Device string  `json:"device"`
	Code   string  `json:"code"`
	Kind   string  `json:"kind"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Speed  int     `json:"speed"`
	Debug  uint32  `json:"debug"`
	Value  float64 `json:"value"`
}

// ---------------------------------------------------------------------------
// Loops
// ---------------------------------------------------------------------------

func (h *hub) run(ctx context.Context) {
	simTicker := time.NewTicker(time.Second / time.Duration(h.simHz))
	renderTicker := time.NewTicker(time.Second / time.Duration(h.fps))
	defer simTicker.Stop()
	defer renderTicker.Stop()

	go h.simulationLoop(ctx, simTicker.C, 1.0/float64(h.simHz))
	h.renderLoop(ctx, renderTicker.C)
}

func (h *hub) simulationLoop(ctx context.Context, simTick <-chan time.Time, dt float64) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-simTick:
			if h.clientCount() == 0 {
				continue
			}

			// drains the inbox atomically — fast operation, only copies slices
			h.inboxMu.Lock()
			events := append([]input.Event(nil), h.pendingEvents...)
			cmds := append([]runtime.ControlCommand(nil), h.pendingCmds...)
			h.pendingEvents = h.pendingEvents[:0]
			h.pendingCmds = h.pendingCmds[:0]
			h.inboxMu.Unlock()

			// The driver is lock-free; the hub serializes access via worldMu.
			h.worldMu.Lock()
			for _, cmd := range cmds {
				h.driver.Control(cmd)
			}
			h.driver.ApplyInput(events)
			h.driver.Advance(dt)
			h.worldMu.Unlock()
		}
	}
}

func (h *hub) renderLoop(ctx context.Context, renderTicker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-renderTicker:
			if h.clientCount() == 0 {
				continue
			}

			// RLock: can contend with other readers, never with Advance
			h.worldMu.RLock()
			frame := h.driver.Frame()
			state := h.driver.State()
			h.worldMu.RUnlock()

			h.broadcast(frame.Commands, frame.Camera, state)
		}
	}
}

// ---------------------------------------------------------------------------
// WebSocket
// ---------------------------------------------------------------------------

func (h *hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}

	c := &client{conn: conn, camera: h.camera}
	h.addClient(c)
	defer func() {
		h.removeClient(c)
		_ = conn.CloseNow()
	}()

	h.readClientMessages(r.Context(), c)
}

func (h *hub) readClientMessages(ctx context.Context, c *client) {
	for {
		var msg clientMessage
		if err := wsjson.Read(ctx, c.conn, &msg); err != nil {
			return
		}

		switch msg.Type {
		case "resize":
			c.applyResize(msg)
		case "control":
			h.enqueueControl(msg)
		case "input":
			h.enqueueInput(msg)
		}
	}
}

func (h *hub) enqueueControl(msg clientMessage) {
	cmd, ok := controlCommandFromMessage(msg)
	if !ok {
		return
	}

	h.inboxMu.Lock()
	h.pendingCmds = append(h.pendingCmds, cmd)
	h.inboxMu.Unlock()
}

func (h *hub) enqueueInput(msg clientMessage) {
	event, ok := inputEventFromClientMessage(msg)
	if !ok {
		return
	}
	h.inboxMu.Lock()
	h.pendingEvents = append(h.pendingEvents, event)
	h.inboxMu.Unlock()
}

// inputEventFromClientMessage translates the browser's JSON envelope into input.Event.
// Web-specific knowledge: device/kind names come from JS; BrowserKeyboardCode
// maps browser key strings to input.SourceCode.
func inputEventFromClientMessage(msg clientMessage) (input.Event, bool) {
	if msg.Device != "keyboard" {
		return input.Event{}, false
	}

	code, ok := BrowserKeyboardCode(msg.Code)
	if !ok {
		return input.Event{}, false
	}

	kind, ok := browserInputKind(msg.Kind)
	if !ok {
		return input.Event{}, false
	}

	event := input.Event{
		Source: input.Source{Device: input.DeviceKeyboard, Code: code},
		Kind:   kind,
		Value:  msg.Value,
	}
	return event, event.Valid()
}

func browserInputKind(kind string) (input.EventKind, bool) {
	switch kind {
	case "down":
		return input.EventButtonDown, true
	case "up":
		return input.EventButtonUp, true
	default:
		return input.EventButtonDown, false
	}
}

// controlCommandFromMessage translates the browser action string into a ControlCommand.
// Web-specific knowledge: action names come from JS, not the runtime.
func controlCommandFromMessage(msg clientMessage) (runtime.ControlCommand, bool) {
	switch msg.Action {
	case "play":
		return runtime.ControlCommand{Kind: runtime.ControlPlay}, true
	case "pause":
		return runtime.ControlCommand{Kind: runtime.ControlPause}, true
	case "step":
		return runtime.ControlCommand{Kind: runtime.ControlStep}, true
	case "back":
		return runtime.ControlCommand{Kind: runtime.ControlBack}, true
	case "reset":
		return runtime.ControlCommand{Kind: runtime.ControlReset}, true
	case "setSpeed":
		return runtime.ControlCommand{Kind: runtime.ControlSetSpeed, Speed: msg.Speed}, true
	case "setDebug":
		return runtime.ControlCommand{
			Kind:        runtime.ControlSetDebugLayers,
			DebugLayers: scene.DebugLayers(msg.Debug),
		}, true
	}
	return runtime.ControlCommand{}, false
}

// ---------------------------------------------------------------------------
// Broadcast
// ---------------------------------------------------------------------------

func (h *hub) broadcast(commands []render.Command, camera render.Camera, state runtime.DriveState) {
	h.clientsMu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.clientsMu.RUnlock()

	for _, c := range clients {
		frame := web.NewFrameDTOWithState(
			state.Tick,
			h.clientCamera(camera, c),
			h.clear,
			web.StateDTO{
				Paused:       state.Paused,
				Speed:        state.Speed,
				Debug:        uint32(state.DebugLayers),
				HistoryLen:   state.HistoryLen,
				RenderFPS:    h.fps,
				SimulationHz: h.simHz,
			},
			commands,
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := wsjson.Write(ctx, c.conn, frame)
		cancel()

		if err != nil {
			h.sendDisconnectFrame(c)
			h.removeClient(c)
			_ = c.conn.CloseNow()
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *hub) clientCamera(base render.Camera, c *client) render.Camera {
	c.mu.RLock()
	viewport := c.camera.Viewport
	c.mu.RUnlock()

	base.Viewport = viewport
	return base
}

func (c *client) applyResize(msg clientMessage) {
	if msg.Width <= 0 || msg.Height <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.camera.Viewport = math2d.NewRect(
		math2d.Vec2{},
		math2d.Vec2{X: msg.Width, Y: msg.Height},
	)
}

func (h *hub) addClient(c *client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *hub) removeClient(c *client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	delete(h.clients, c)
}

func (h *hub) clientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

func (h *hub) sendDisconnectFrame(c *client) {
	c.mu.RLock()
	viewport := c.camera.Viewport
	c.mu.RUnlock()

	cam := render.Camera{
		World:    viewport,
		Viewport: viewport,
		Aspect:   render.AspectStretch,
	}

	commands := []render.Command{
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 999},
			Position:    viewport.Center(),
			Text:        "disconnected, retrying...",
			Color:       render.RGBA(1, 1, 1, 1),
			Size:        24,
		},
	}

	frame := web.NewFrameDTOWithState(0, cam, h.clear, web.StateDTO{}, commands)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_ = wsjson.Write(ctx, c.conn, frame)
	cancel()
}

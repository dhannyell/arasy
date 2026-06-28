package canvas

import (
	"context"
	"embed"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	adapter "github.com/dhannyell/frameforge/render/adapter/web"
)

const maxStepsPerFrame = 5

type Server struct {
	Addr         string
	FPS          int
	SimulationHz int

	Camera         render.Camera
	CameraForFrame func() render.Camera
	Clear          render.Color

	Update     func(dt float64)
	Commands   func(debug bool) []render.Command
	Input      func(state input.State)
	HistoryLen func() int
	Reset      func()
	Back       func() bool

	InputBindings []input.Binding
}

type clientMessage struct {
	Type   string `json:"type"`
	Action string `json:"action"`
	Device string `json:"device"`
	Code   string `json:"code"`
	Kind   string `json:"kind"`

	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Speed  int     `json:"speed"`
	Debug  bool    `json:"debug"`
	Value  float64 `json:"value"`
}

type client struct {
	conn *websocket.Conn

	mu     sync.RWMutex
	camera render.Camera
}

type controlState struct {
	mu sync.Mutex

	paused      bool
	speed       int
	debug       bool
	stepPending int
	reset       bool
	backPending int
	inputEvents []input.Event
}

type frameState struct {
	paused      bool
	speed       int
	debug       bool
	reset       bool
	steps       int
	backs       int
	inputEvents []input.Event
}

//go:embed static
var staticFiles embed.FS

func (s *Server) ListenAndServe() error {
	fps := s.FPS
	if fps <= 0 {
		fps = 60
	}

	simulationHz := s.SimulationHz
	if simulationHz <= 0 {
		simulationHz = fps
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := newHub(s, fps, simulationHz)
	go hub.run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ws", hub.handleWS)
	mux.Handle("/static/", http.FileServerFS(staticFiles))

	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFileFS(w, r, staticFiles, "static/index.html")
}

type hub struct {
	server       *Server
	fps          int
	simulationHz int

	controls controlState

	inputMapper *input.Mapper
	inputBuffer *input.Buffer

	clientsMu sync.RWMutex
	clients   map[*client]struct{}

	// Shared state between simulation and render loops
	stateMu  sync.RWMutex
	worldMu  sync.RWMutex // Protects World access (Update/Commands)
	lastTick uint64
}

type renderFrame struct {
	tick     uint64
	commands []render.Command
	state    frameState
}

func newHub(server *Server, fps int, simulationHz int) *hub {
	return &hub{
		server:       server,
		fps:          fps,
		simulationHz: simulationHz,
		controls: controlState{
			speed: 1,
			debug: true,
		},
		inputMapper: input.NewMapper(server.InputBindings),
		inputBuffer: input.NewBuffer(),
		clients:     make(map[*client]struct{}),
	}
}

func (h *hub) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}

	c := &client{
		conn:   conn,
		camera: h.server.Camera,
	}

	h.addClient(c)
	defer func() {
		h.removeClient(c)
		_ = conn.CloseNow()
	}()

	h.readClientMessages(r.Context(), c)
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

func (h *hub) readClientMessages(ctx context.Context, c *client) {
	for {
		var message clientMessage
		if err := wsjson.Read(ctx, c.conn, &message); err != nil {
			return
		}

		switch message.Type {
		case "resize":
			c.applyResize(message)
		case "control":
			h.controls.apply(message)
		case "input":
			h.controls.applyInput(message)
		}
	}
}

func (c *client) applyResize(message clientMessage) {
	if message.Width <= 0 || message.Height <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.camera.Viewport = math2d.NewRect(
		math2d.Vec2{X: 0, Y: 0},
		math2d.Vec2{X: message.Width, Y: message.Height},
	)
}

func (c *client) frameCamera() render.Camera {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.camera
}

func (h *hub) frameCamera(c *client) render.Camera {
	clientCamera := c.frameCamera()

	if h.server.CameraForFrame == nil {
		return clientCamera
	}

	camera := h.server.CameraForFrame()
	camera.Viewport = clientCamera.Viewport

	return camera
}

func (h *hub) run(ctx context.Context) {
	simulationDT := 1.0 / float64(h.simulationHz)

	simTicker := time.NewTicker(time.Second / time.Duration(h.simulationHz))
	defer simTicker.Stop()

	renderTicker := time.NewTicker(time.Second / time.Duration(h.fps))
	defer renderTicker.Stop()

	frameChan := make(chan renderFrame, 1)
	defer close(frameChan)

	// Run simulation loop in separate goroutine
	go h.simulationLoop(ctx, simTicker.C, simulationDT, frameChan)

	// Run render loop in main goroutine
	h.renderLoop(ctx, renderTicker.C, frameChan)
}

func (h *hub) simulationLoop(ctx context.Context, simTick <-chan time.Time, simulationDT float64, frameChan chan<- renderFrame) {
	var tick uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-simTick:
			if h.clientCount() == 0 {
				continue
			}

			state := h.controls.consumeFrameState()
			h.applyInputEvents(tick, state.inputEvents)

			h.worldMu.Lock()

			if state.reset {
				if h.server.Reset != nil {
					h.server.Reset()
				}
				tick = 0
			}

			for range state.backs {
				if h.server.Back != nil && h.server.Back() && tick > 0 {
					tick--
				}
			}

			if !(state.paused && state.steps == 0) {
				stepsToRun := state.steps
				if stepsToRun == 0 && !state.paused {
					stepsToRun = 1
				}

				for range stepsToRun {
					if h.server.Input != nil {
						h.server.Input(h.inputMapper.State())
					}
					if h.server.Update != nil {
						h.server.Update(simulationDT)
					}
					tick++
					h.inputMapper.EndFrame()
					h.inputBuffer.ClearBefore(retainedInputTick(tick, 120))
				}
			}

			h.worldMu.Unlock()

			h.stateMu.Lock()
			h.lastTick = tick
			h.stateMu.Unlock()

			// Send frame for rendering (non-blocking, drop if render is slow)
			select {
			case frameChan <- renderFrame{tick: tick, state: state}:
			default:
			}
		}
	}
}

func (h *hub) renderLoop(ctx context.Context, renderTick <-chan time.Time, frameChan <-chan renderFrame) {
	var lastFrameState frameState

	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-frameChan:
			lastFrameState = frame.state
		case <-renderTick:
			if h.clientCount() == 0 {
				continue
			}

			h.stateMu.RLock()
			tick := h.lastTick
			h.stateMu.RUnlock()

			var commands []render.Command
			h.worldMu.RLock()
			if h.server.Commands != nil {
				commands = h.server.Commands(lastFrameState.debug)
			}
			h.worldMu.RUnlock()

			h.broadcast(tick, commands, lastFrameState)
		}
	}
}

func nextSimulationSteps(
	manualSteps int,
	speed int,
	renderDT float64,
	simulationDT float64,
	accumulator *float64,
) int {
	if manualSteps > 0 {
		return manualSteps
	}

	if speed <= 0 || simulationDT <= 0 || renderDT <= 0 {
		return 0
	}

	*accumulator += renderDT * float64(speed)
	steps := int(math.Floor(*accumulator / simulationDT))

	if steps > maxStepsPerFrame {
		steps = maxStepsPerFrame
		*accumulator = 0
	} else {
		*accumulator -= float64(steps) * simulationDT
	}

	return steps
}

func (h *hub) applyInputEvents(tick uint64, events []input.Event) {
	for _, event := range events {
		event.Tick = tick
		h.inputBuffer.Push(event)
		h.inputMapper.Apply(event)
	}
}

func retainedInputTick(tick uint64, retainedTicks uint64) uint64 {
	if tick <= retainedTicks {
		return 0
	}

	return tick - retainedTicks
}

func (h *hub) clientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	return len(h.clients)
}

func (h *hub) broadcast(tick uint64, commands []render.Command, state frameState) {
	h.clientsMu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.clientsMu.RUnlock()

	for _, c := range clients {
		frame := adapter.NewFrameDTOWithState(
			tick,
			h.frameCamera(c),
			h.server.Clear,
			adapter.StateDTO{
				Paused:       state.paused,
				Speed:        h.controls.currentSpeed(),
				Debug:        state.debug,
				HistoryLen:   h.historyLen(),
				RenderFPS:    h.fps,
				SimulationHz: h.simulationHz,
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

func (h *hub) sendDisconnectFrame(c *client) {
	camera := c.frameCamera()
	viewport := camera.Viewport

	identityCamera := render.Camera{
		World:    viewport,
		Viewport: viewport,
		Aspect:   render.AspectStretch,
	}

	center := viewport.Center()

	commands := []render.Command{
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 999},
			Position:    center,
			Text:        "disconnected, retrying...",
			Color:       render.RGBA(1, 1, 1, 1),
			Size:        24,
		},
	}

	frame := adapter.NewFrameDTOWithState(
		0,
		identityCamera,
		h.server.Clear,
		adapter.StateDTO{},
		commands,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_ = wsjson.Write(ctx, c.conn, frame)
	cancel()
}

func (h *hub) historyLen() int {
	if h.server.HistoryLen == nil {
		return 0
	}

	return h.server.HistoryLen()
}

func (s *controlState) apply(message clientMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch message.Action {
	case "play":
		s.paused = false
	case "pause":
		s.paused = true
	case "step":
		s.paused = true
		s.stepPending++
	case "back":
		s.paused = true
		s.backPending++
	case "reset":
		s.reset = true
		s.stepPending = 0
		s.backPending = 0
	case "setSpeed":
		if message.Speed < 1 {
			message.Speed = 1
		}
		if message.Speed > 8 {
			message.Speed = 8
		}
		s.speed = message.Speed
	case "setDebug":
		s.debug = message.Debug
	}
}

func (s *controlState) applyInput(message clientMessage) {
	event, ok := inputEventFromClientMessage(message)
	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.inputEvents = append(s.inputEvents, event)
}

func inputEventFromClientMessage(message clientMessage) (input.Event, bool) {
	if message.Device != "keyboard" {
		return input.Event{}, false
	}

	code, ok := BrowserKeyboardCode(message.Code)
	if !ok {
		return input.Event{}, false
	}

	kind, ok := browserInputKind(message.Kind)
	if !ok {
		return input.Event{}, false
	}

	event := input.Event{
		Source: input.Source{
			Device: input.DeviceKeyboard,
			Code:   code,
		},
		Kind:  kind,
		Value: message.Value,
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

func (s *controlState) consumeFrameState() frameState {
	s.mu.Lock()
	defer s.mu.Unlock()

	speed := s.speed
	if s.paused {
		speed = 0
	}

	steps := s.stepPending
	s.stepPending = 0

	backs := s.backPending
	s.backPending = 0

	reset := s.reset
	s.reset = false

	inputEvents := append([]input.Event(nil), s.inputEvents...)
	s.inputEvents = s.inputEvents[:0]

	return frameState{
		paused:      s.paused,
		speed:       speed,
		debug:       s.debug,
		reset:       reset,
		steps:       steps,
		backs:       backs,
		inputEvents: inputEvents,
	}
}

func (s *controlState) currentSpeed() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.speed
}

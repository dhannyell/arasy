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
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	adapter "github.com/dhannyell/frameforge/render/adapter/web"
)

type Server struct {
	Addr         string
	FPS          int
	SimulationHz int

	Camera render.Camera
	Clear  render.Color

	Update   func(dt float64)
	Commands func(debug bool) []render.Command
	Reset    func()
	Back     func() bool
}

type clientMessage struct {
	Type   string `json:"type"`
	Action string `json:"action"`

	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Speed  int     `json:"speed"`
	Debug  bool    `json:"debug"`
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
}

type frameState struct {
	speed int
	debug bool
	reset bool
	steps int
	backs int
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

	hub := newHub(s, fps, simulationHz)
	go hub.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ws", hub.handleWS)
	mux.Handle("/static/", http.FileServerFS(staticFiles))

	return http.ListenAndServe(s.Addr, mux)
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

	clientsMu sync.RWMutex
	clients   map[*client]struct{}
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
		clients: make(map[*client]struct{}),
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

func (h *hub) run() {
	renderDT := 1.0 / float64(h.fps)
	simulationDT := 1.0 / float64(h.simulationHz)
	ticker := time.NewTicker(time.Second / time.Duration(h.fps))
	defer ticker.Stop()

	var tick uint64
	var accumulator float64

	for range ticker.C {
		if h.clientCount() == 0 {
			accumulator = 0
			continue
		}

		state := h.controls.consumeFrameState()

		if state.reset {
			if h.server.Reset != nil {
				h.server.Reset()
			}
			tick = 0
			accumulator = 0
		}

		for range state.backs {
			if h.server.Back != nil && h.server.Back() && tick > 0 {
				tick--
			}
		}

		steps := state.steps
		if steps == 0 && state.speed > 0 {
			accumulator += renderDT * float64(state.speed)
			steps = int(math.Floor(accumulator / simulationDT))
			accumulator -= float64(steps) * simulationDT
		}

		for range steps {
			if h.server.Update != nil {
				h.server.Update(simulationDT)
			}
			tick++
		}

		var commands []render.Command
		if h.server.Commands != nil {
			commands = h.server.Commands(state.debug)
		}

		h.broadcast(tick, commands)
	}
}

func (h *hub) clientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	return len(h.clients)
}

func (h *hub) broadcast(tick uint64, commands []render.Command) {
	h.clientsMu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.clientsMu.RUnlock()

	for _, c := range clients {
		frame := adapter.NewFrameDTO(tick, c.frameCamera(), h.server.Clear, commands)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := wsjson.Write(ctx, c.conn, frame)
		cancel()

		if err != nil {
			h.removeClient(c)
			_ = c.conn.CloseNow()
		}
	}
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

	return frameState{
		speed: speed,
		debug: s.debug,
		reset: reset,
		steps: steps,
		backs: backs,
	}
}

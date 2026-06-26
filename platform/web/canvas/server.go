package canvas

import (
	"context"
	"embed"
	"fmt"
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

type controlState struct {
	mu sync.RWMutex

	paused      bool
	speed       int
	debug       bool
	stepPending int
	reset       bool
	backPending int
}

type frameState struct {
	steps int
	debug bool
	reset bool
	backs int
}

//go:embed static
var staticFiles embed.FS

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func(conn *websocket.Conn) {
		err := conn.CloseNow()
		if err != nil {
			return
		}
	}(conn)

	fps := s.FPS
	if fps <= 0 {
		fps = 60
	}

	simulationHz := s.SimulationHz
	if simulationHz <= 0 {
		simulationHz = fps
	}

	camera := s.Camera
	var cameraMu sync.RWMutex
	controls := controlState{
		speed: 1,
		debug: true,
	}

	go readClientMessages(r.Context(), conn, &cameraMu, &camera, &controls)

	dt := 1.0 / float64(simulationHz)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	var tick uint64
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			state := controls.consumeFrameState()
			if state.reset {
				if s.Reset != nil {
					s.Reset()
				}
				tick = 0
			}

			for range state.backs {
				if s.Back != nil && s.Back() && tick > 0 {
					tick--
				}
			}

			for range state.steps {
				if s.Update != nil {
					s.Update(dt)
				}
				tick++
			}

			var commands []render.Command
			if s.Commands != nil {
				commands = s.Commands(state.debug)
			}

			cameraMu.RLock()
			frameCamera := camera
			cameraMu.RUnlock()

			frame := adapter.NewFrameDTO(tick, frameCamera, s.Clear, commands)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err := wsjson.Write(ctx, conn, frame)
			cancel()

			if err != nil {
				return
			}
		}
	}
}

func readClientMessages(
	ctx context.Context,
	conn *websocket.Conn,
	cameraMu *sync.RWMutex,
	camera *render.Camera,
	controls *controlState,
) {
	for {
		var message clientMessage
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			return
		}

		switch message.Type {
		case "resize":
			if message.Width <= 0 || message.Height <= 0 {
				continue
			}

			cameraMu.Lock()
			camera.Viewport = math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: message.Width, Y: message.Height},
			)
			cameraMu.Unlock()
		case "control":
			controls.apply(message)
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

	steps := s.speed
	if s.paused {
		steps = 0
	}

	if s.stepPending > 0 {
		steps = s.stepPending
		s.stepPending = 0
	}

	backs := s.backPending
	s.backPending = 0

	reset := s.reset
	s.reset = false

	return frameState{
		steps: steps,
		debug: s.debug,
		reset: reset,
		backs: backs,
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFileFS(w, r, staticFiles, "static/index.html")
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ws", s.handleWS)

	mux.Handle(
		"/static/",
		http.FileServerFS(staticFiles),
	)

	fmt.Println("Server is up and running")
	return http.ListenAndServe(s.Addr, mux)
}

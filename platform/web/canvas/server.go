package canvas

import (
	"context"
	"embed"
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
	Addr   string
	FPS    int
	Camera render.Camera
	Clear  render.Color
	Frame  func(tick uint64, dt float64) []render.Command
}

type clientMessage struct {
	Type   string  `json:"type"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
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

	camera := s.Camera
	var cameraMu sync.RWMutex

	go readClientMessages(r.Context(), conn, &cameraMu, &camera)

	dt := 1.0 / float64(fps)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for tick := uint64(0); true; tick++ {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			commands := s.Frame(tick, dt)

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
) {
	for {
		var message clientMessage
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			return
		}

		if message.Type != "resize" || message.Width <= 0 || message.Height <= 0 {
			continue
		}

		cameraMu.Lock()
		camera.Viewport = math2d.NewRect(
			math2d.Vec2{X: 0, Y: 0},
			math2d.Vec2{X: message.Width, Y: message.Height},
		)
		cameraMu.Unlock()
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

	return http.ListenAndServe(s.Addr, mux)
}

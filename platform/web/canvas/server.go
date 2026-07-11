package canvas

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/web/browserinput"
	"github.com/dhannyell/frameforge/platform/web/canvas/internal/dto"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

//go:embed static
var staticFiles embed.FS

type Options struct {
	Addr string
	Log  *slog.Logger
	// filesystem mounted at /audio/ clip paths in the game config
	// are relative to it; nil disables audio delivery
	AudioFS fs.FS
}

func Run(game runtime.Game, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
		opts.Log = log
	}
	driver := runtime.NewDriver(game, log)
	return RunDriver(context.Background(), driver, opts)
}

func RunDriver(ctx context.Context, driver *runtime.Driver, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	cfg := driver.Config()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h := &hub{
		driver:             driver,
		fps:                cfg.FPS,
		simHz:              cfg.SimulationHz,
		clear:              cfg.Clear,
		camera:             cfg.Camera,
		assets:             cfg.Assets,
		textureVersion:     make(map[render.TextureID]int),
		dynamicTextureData: initDynamicTextures(cfg.Assets),
		clients:            make(map[*client]struct{}),
	}
	if opts.AudioFS != nil {
		h.audioEnabled = true
		h.audioManifest = newAudioManifestDTO(cfg.Assets.AudioClips())
	}
	go h.run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/ws", h.handleWS)
	mux.Handle("/static/", http.FileServerFS(staticFiles))
	if opts.AudioFS != nil {
		mux.Handle("/audio/", http.StripPrefix("/audio/", http.FileServerFS(opts.AudioFS)))
	}
	mux.HandleFunc("/assets/manifest", h.handleAssetManifest)
	mux.HandleFunc("/assets/textures/", h.handleAssetTexture)
	mux.HandleFunc("/assets/dynamic/", h.handleDynamicTexture)

	srv := &http.Server{
		Addr:              opts.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Info("canvas listening", "addr", opts.Addr)
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
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
	inboxMu         sync.Mutex
	pendingEvents   []input.Event
	pendingCmds     []runtime.ControlCommand
	pendingViewport math2d.Rect
	haveViewport    bool

	// worldMu: AdvanceTime exclusivo, Commands compartilhado
	worldMu sync.RWMutex

	clientsMu sync.RWMutex
	clients   map[*client]struct{}

	audioMu         sync.Mutex
	audioEnabled    bool
	audioManifest   audioManifestDTO
	lastAudioTracks map[string]audioTrackDTO
	pendingAudio    []audioMessageDTO

	assets             asset.Manifest
	textureVersion     map[render.TextureID]int
	pendingTexture     []textureUpdateDTO
	textureVersionMu   sync.Mutex
	dynamicTextureData map[render.TextureID][]byte
	dynamicTextureMu   sync.RWMutex
}

type client struct {
	conn   *websocket.Conn
	mu     sync.RWMutex
	camera render.Camera
}

// clientMessage is the JSON envelope sent by the browser.
type clientMessage struct {
	Type    string  `json:"type"`
	Action  string  `json:"action"`
	Device  string  `json:"device"`
	Code    string  `json:"code"`
	Kind    string  `json:"kind"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Speed   float64 `json:"speed"`
	Debug   uint32  `json:"debug"`
	Tick    uint64  `json:"tick"`
	Seconds float64 `json:"seconds"`
	Value   float64 `json:"value"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

// assetManifestDTO lists the declared textures so the browser can
// prefetch and register them by ID before frames arrive.
type assetManifestDTO struct {
	Textures []assetTextureDTO `json:"textures"`
}

type assetTextureDTO struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (h *hub) handleAssetManifest(w http.ResponseWriter, r *http.Request) {
	decls := h.assets.Textures()
	dto := assetManifestDTO{Textures: make([]assetTextureDTO, 0, len(decls))}
	for _, d := range decls {
		url := fmt.Sprintf("/assets/textures/%d", d.ID)
		if d.Dynamic {
			url = fmt.Sprintf("/assets/dynamic/%d", d.ID)
		}
		dto.Textures = append(dto.Textures, assetTextureDTO{
			ID:   uint32(d.ID),
			Name: d.Name,
			URL:  url,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // IDs are per-run
	_ = json.NewEncoder(w).Encode(dto)
}

func (h *hub) handleAssetTexture(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/assets/textures/")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	for _, d := range h.assets.Textures() {
		if d.Dynamic {
			continue
		}
		if uint32(d.ID) == uint32(id) {
			w.Header().Set("Content-Type", http.DetectContentType(d.Data))
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(d.Data)
			return
		}
	}
	http.NotFound(w, r)
}

// ---------------------------------------------------------------------------
// Loops
// ---------------------------------------------------------------------------

func (h *hub) run(ctx context.Context) {
	simTicker := time.NewTicker(time.Second / time.Duration(h.simHz))
	renderTicker := time.NewTicker(time.Second / time.Duration(h.fps))
	defer simTicker.Stop()
	defer renderTicker.Stop()

	go h.simulationLoop(ctx, simTicker.C)
	h.renderLoop(ctx, renderTicker.C)
}

func (h *hub) simulationLoop(ctx context.Context, simTick <-chan time.Time) {
	var frameTimer runtime.FrameTimer

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-simTick:
			if h.clientCount() == 0 {
				frameTimer.Reset()
				continue
			}

			elapsed := frameTimer.Elapsed(now)

			events, cmds, viewport, haveViewport := h.drainInbox()

			// The driver is lock-free; the hub serializes access via worldMu.
			h.worldMu.Lock()
			var viewportPtr *math2d.Rect
			if haveViewport {
				viewportPtr = &viewport
			}
			_, out := h.driver.RunTick(runtime.TickBatch{
				Viewport: viewportPtr,
				Controls: cmds,
				Events:   events,
			}, elapsed)
			texOut := h.driver.ConsumeTextureUpdates()
			h.worldMu.Unlock()

			h.enqueueAudio(out)
			h.enqueueTextureUpdates(texOut)
		}
	}
}

func (h *hub) enqueueAudio(out audio.Output) {
	if !h.audioEnabled {
		return
	}
	msg := newAudioMessageDTO(out)

	h.audioMu.Lock()
	defer h.audioMu.Unlock()

	tracksChanged := !maps.Equal(h.lastAudioTracks, msg.Tracks)
	if !tracksChanged && len(msg.Cues) == 0 {
		return
	}
	if tracksChanged {
		h.lastAudioTracks = cloneAudioTrackDTOs(msg.Tracks)
	} else {
		msg.Tracks = nil
	}
	h.pendingAudio = append(h.pendingAudio, msg)
}

func (h *hub) drainInbox() ([]input.Event, []runtime.ControlCommand, math2d.Rect, bool) {
	// Drains the inbox atomically. Slices are copied so handlers can keep
	// appending while the simulation owns the returned batch.
	h.inboxMu.Lock()
	defer h.inboxMu.Unlock()

	events := append([]input.Event(nil), h.pendingEvents...)
	cmds := append([]runtime.ControlCommand(nil), h.pendingCmds...)
	viewport := h.pendingViewport
	haveViewport := h.haveViewport

	h.pendingEvents = h.pendingEvents[:0]
	h.pendingCmds = h.pendingCmds[:0]
	h.haveViewport = false

	return events, cmds, viewport, haveViewport
}

func (h *hub) renderLoop(ctx context.Context, renderTicker <-chan time.Time) {
	var frameTimer runtime.FrameTimer

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-renderTicker:
			if h.clientCount() == 0 {
				frameTimer.Reset()
				continue
			}

			elapsed := frameTimer.Elapsed(now)

			// RLock: can contend with other readers, never with Advance
			h.worldMu.RLock()
			frame := h.driver.Frame(elapsed)
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
	h.sendAudioBootstrap(r.Context(), c)
	defer func() {
		h.removeClient(c)
		_ = conn.CloseNow()
	}()

	h.readClientMessages(r.Context(), c)
}

func (h *hub) sendAudioBootstrap(ctx context.Context, c *client) {
	h.audioMu.Lock()
	if !h.audioEnabled {
		h.audioMu.Unlock()
		return
	}
	manifest := h.audioManifest
	tracks := cloneAudioTrackDTOs(h.lastAudioTracks)
	h.audioMu.Unlock()

	if len(manifest.Clips) > 0 {
		if !writeWS(ctx, c, manifest) {
			return
		}
	}
	if len(tracks) > 0 {
		_ = writeWS(ctx, c, audioMessageDTO{Type: "audio", Tracks: tracks})
	}
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
			h.enqueueResize(msg)
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

func (h *hub) enqueueResize(msg clientMessage) {
	if msg.Width <= 0 || msg.Height <= 0 {
		return
	}
	h.inboxMu.Lock()
	h.pendingViewport = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: msg.Width, Y: msg.Height})
	h.haveViewport = true
	h.inboxMu.Unlock()
}

// inputEventFromClientMessage maps browser input JSON to input.Event.
func inputEventFromClientMessage(msg clientMessage) (input.Event, bool) {
	switch msg.Device {
	case "keyboard":
		event, ok := browserinput.KeyEvent(msg.Code, msg.Kind == "down")
		if !ok {
			return input.Event{}, false
		}
		if msg.Kind != "down" && msg.Kind != "up" {
			return input.Event{}, false
		}
		event.Value = msg.Value
		return event, event.Valid()
	case "mouse":
		return mouseEventFromMessage(msg)
	default:
		return input.Event{}, false
	}
}

func mouseEventFromMessage(msg clientMessage) (input.Event, bool) {
	switch msg.Code {
	case "Position":
		return browserinput.MouseMoveEvent(msg.X, msg.Y), true
	case "Leave":
		return browserinput.PointerLeaveEvent(), true
	case "WheelX", "WheelY":
		dx, dy := 0.0, 0.0
		if msg.Code == "WheelX" {
			dx = msg.Value
		} else {
			dy = msg.Value
		}
		events := browserinput.WheelEvent(dx, dy)
		if len(events) != 1 {
			return input.Event{}, false
		}
		event := events[0]
		event.X, event.Y = msg.X, msg.Y
		return event, event.Valid()
	default:
		if msg.Kind != "down" && msg.Kind != "up" {
			return input.Event{}, false
		}
		return browserinput.MouseButtonEventCode(msg.Code, msg.Kind == "down", msg.X, msg.Y)
	}
}

// controlCommandFromMessage maps browser action strings to runtime controls.
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
	case "backToTick":
		return runtime.ControlCommand{Kind: runtime.ControlBackToTick, Tick: msg.Tick}, true
	case "scrubToTick":
		return runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: msg.Tick}, true
	case "backSeconds":
		return runtime.ControlCommand{Kind: runtime.ControlBackSeconds, Seconds: msg.Seconds}, true
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
		frame := dto.NewFrameDTOWithState(
			state.Tick,
			h.clientCamera(camera, c),
			h.clear,
			dto.StateDTO{
				Paused:           state.Paused,
				Speed:            state.Speed,
				Debug:            uint32(state.DebugLayers),
				HistoryLen:       state.HistoryLen,
				HistoryStartTick: state.HistoryStartTick,
				HistoryEndTick:   state.HistoryEndTick,
				MaxRecordedTick:  state.MaxRecordedTick,
				RenderFPS:        h.fps,
				SimulationHz:     h.simHz,
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

	for _, msg := range h.drainAudio() {
		for _, c := range clients {
			if !writeWS(context.Background(), c, msg) {
				h.sendDisconnectFrame(c)
				h.removeClient(c)
				_ = c.conn.CloseNow()
			}
		}
	}

	if updates := h.drainTextureUpdates(); len(updates) > 0 {
		msg := textureUpdateListDTO{Type: "textureUpdate", Updates: updates}
		for _, c := range clients {
			if !writeWS(context.Background(), c, msg) {
				h.sendDisconnectFrame(c)
				h.removeClient(c)
				_ = c.conn.CloseNow()
			}
		}
	}
}

func (h *hub) drainAudio() []audioMessageDTO {
	h.audioMu.Lock()
	defer h.audioMu.Unlock()

	out := append([]audioMessageDTO(nil), h.pendingAudio...)
	h.pendingAudio = h.pendingAudio[:0]
	return out
}

func writeWS(ctx context.Context, c *client, v any) bool {
	writeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return wsjson.Write(writeCtx, c.conn, v) == nil
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

	frame := dto.NewFrameDTOWithState(0, cam, h.clear, dto.StateDTO{}, commands)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_ = wsjson.Write(ctx, c.conn, frame)
	cancel()
}

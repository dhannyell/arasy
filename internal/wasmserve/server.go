// Package wasmserve create a simple HTTP server to run a compiled wasm file in browser
package wasmserve

import (
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// posterSVG is the engine brand's static boot poster — the same rest-pose
// composition internal/enginesplash's compiled animation opens on, with a
// frozen frame of its own starfield/Crux particles baked in — embedded in
// the CLI binary so the shell can paint it inline before any byte of
// app.wasm has arrived. It is vector so it stays crisp at any viewport
// size instead of being raster-scaled, and its wordmark is already plain
// path data, so embedding it carries no font redistribution concern.
//
//go:embed assets/poster.svg
var posterSVG []byte

// posterStackedSVG is the same boot poster, laid out for portrait viewports
// (mark centered above the wordmark instead of beside it) so the mark and
// wordmark stay legible on a phone instead of being letterboxed down to fit
// a canvas designed for landscape. Mirrors internal/enginesplash/stacked's
// compiled animation, which the shell switches to at the same breakpoint
// (see wasmShellHTML's max-aspect-ratio media query).
//
//go:embed assets/poster-stacked.svg
var posterStackedSVG []byte

// posterBackground matches the flat backdrop the poster and the compiled
// engine splash both paint, so the page never shows a mismatched edge around
// the inline image.
const posterBackground = "#020B14"

const (
	PosterLandscapeFile      = "arasy-poster.svg"
	PosterStackedFile        = "arasy-poster-stacked.svg"
	uncompressedLengthHeader = "X-Arasy-Uncompressed-Length"
)

type ServerOptions struct {
	Dir          string
	Title        string
	WASMFile     string
	WASMExecPath string
	Addr         string
	// DisableSplash omits the loading poster and tells the WASM host to skip
	// every runtime splash segment for this served session.
	DisableSplash bool
	// Rebuild runs before serving WASMFile. Development hosts use it to build
	// on browser reload; a failure is returned to the browser as plain text.
	Rebuild func() error
}

func NewServerOptions(dir, title, file, execPath, addr string) *ServerOptions {
	return &ServerOptions{
		Dir:          dir,
		Title:        title,
		WASMFile:     file,
		WASMExecPath: execPath,
		Addr:         addr,
	}
}

func Start(opts *ServerOptions) error {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if opts.WASMExecPath == "" {
		var err error
		opts.WASMExecPath, err = DefaultWASMExecPath()
		if err != nil {
			log.Error("wasm_exec.js not found", "err", err)
			return err
		}
	}

	handler := newHandler(ServerOptions{
		Dir:           opts.Dir,
		Title:         opts.Title,
		WASMFile:      opts.WASMFile,
		WASMExecPath:  opts.WASMExecPath,
		DisableSplash: opts.DisableSplash,
		Rebuild:       opts.Rebuild,
	})

	fs := http.FileServer(http.Dir(opts.Dir))

	log.Info("listening", "addr", opts.Addr, "dir", opts.Dir, "wasm", opts.WASMFile, "wasm_exec", opts.WASMExecPath)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")

		fs.ServeHTTP(w, r)
	})
	if err := http.ListenAndServe(opts.Addr, gzipHandler(handler)); err != nil {
		log.Error("server failed", "err", err)
		return err
	}
	return nil
}

func newHandler(opts ServerOptions) http.Handler {
	files := http.FileServer(http.Dir(opts.Dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			serveIndex(w, opts)
		case "/wasm_exec.js":
			http.ServeFile(w, r, opts.WASMExecPath)
		case "/" + PosterLandscapeFile:
			servePoster(w, posterSVG)
		case "/" + PosterStackedFile:
			servePoster(w, posterStackedSVG)
		default:
			if r.URL.Path == "/"+opts.WASMFile && opts.Rebuild != nil {
				if err := opts.Rebuild(); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			files.ServeHTTP(w, r)
		}
	})
}

func gzipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" ||
			!acceptsEncoding(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		writer := &gzipResponseWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		if writer.gzip != nil {
			_ = writer.gzip.Close()
		}
	})
}

func acceptsEncoding(header, encoding string) bool {
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		name := strings.TrimSpace(parts[0])
		if !strings.EqualFold(name, encoding) && name != "*" {
			continue
		}
		enabled := true
		for _, parameter := range parts[1:] {
			if strings.EqualFold(strings.TrimSpace(parameter), "q=0") {
				enabled = false
			}
		}
		if enabled {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gzip    *gzip.Writer
	started bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.started {
		return
	}
	w.started = true
	if compressibleContentType(w.Header().Get("Content-Type")) {
		if length := w.Header().Get("Content-Length"); length != "" {
			// Fetch exposes the decoded response body to JavaScript. Preserve
			// its original byte count before replacing Content-Length with the
			// gzip transfer size so the loading bar can measure decoded bytes.
			w.Header().Set(uncompressedLengthHeader, length)
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.gzip = gzip.NewWriter(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.started {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(data))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.gzip != nil {
		return w.gzip.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *gzipResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if !w.started {
		w.WriteHeader(http.StatusOK)
	}
	if w.gzip != nil {
		return io.Copy(w.gzip, reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}

func compressibleContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return strings.HasPrefix(contentType, "text/") ||
		contentType == "application/javascript" ||
		contentType == "application/json" ||
		contentType == "application/wasm" ||
		contentType == "image/svg+xml"
}

func servePoster(w http.ResponseWriter, poster []byte) {
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write(poster)
}

// WritePosterAssets writes the two responsive boot-poster variants beside a
// static build. The browser selects one through <picture>, so both ship in
// the artifact but only the variant matching the viewport is downloaded.
func WritePosterAssets(dir string) error {
	for name, data := range map[string][]byte{
		PosterLandscapeFile: posterSVG,
		PosterStackedFile:   posterStackedSVG,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func serveIndex(w http.ResponseWriter, opts ServerOptions) {
	body, err := shellHTML(opts.Title, opts.WASMFile, opts.DisableSplash)
	if err != nil {
		http.Error(w, "invalid wasm file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(body); err != nil {
		slog.Warn("failed to write index response", "err", err)
	}
}

// ShellHTML returns the browser shell used by both the development server and
// static builds. The canvas stays hidden behind an inline poster + download
// progress bar until the WASM host signals its first presented frame
// (window.arasyFirstFrame = true) — no black canvas flash between navigation
// and the first frame, matching the Unity WebGL / Godot web pattern: static
// content painted by the host page, replaced atomically once the engine can
// draw. Hosts serving app.wasm outside `arasy serve`/`dev`/`build` must
// reproduce the arasyFirstFrame contract themselves.
func ShellHTML(title, wasmFile string) ([]byte, error) {
	return shellHTML(title, wasmFile, false)
}

func shellHTML(title, wasmFile string, disableSplash bool) ([]byte, error) {
	encodedWASMFile, err := json.Marshal(wasmFile)
	if err != nil {
		return nil, err
	}
	canvasVisibility := "hidden"
	bootMarkup := `<div id="ar-boot">
        <picture id="ar-poster-frame">
            <source media="(max-aspect-ratio: 1/1)" srcset="arasy-poster-stacked.svg">
            <img id="ar-poster" src="arasy-poster.svg" alt="Arasy Engine">
        </picture>

        <div id="ar-bar">
			<div id="ar-bar-fill"></div>
		</div>
    </div>`
	if disableSplash {
		canvasVisibility = "visible"
		bootMarkup = ""
	}
	return []byte(fmt.Sprintf(wasmShellHTML,
		html.EscapeString(title), posterBackground, canvasVisibility,
		posterBackground, bootMarkup, disableSplash, encodedWASMFile,
	)), nil
}

// DefaultWASMExecPath finds the Go runtime support script for the Go tool on
// PATH. It deliberately asks that tool instead of relying on runtime.GOROOT.
func DefaultWASMExecPath() (string, error) {
	goroot, err := findGOROOT()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"),
		filepath.Join(goroot, "misc", "wasm", "wasm_exec.js"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("wasm_exec.js not found under GOROOT=%s; checked %s", goroot, candidates)
}

// findGOROOT runs "go env GOROOT" instead of using runtime.GOROOT(),
// which is deprecated since Go 1.24 (the baked-in root is meaningless
// when the binary has been copied to another machine).
func findGOROOT() (string, error) {
	cmd := exec.Command("go", "env", "GOROOT")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOROOT failed: %w", err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("go env GOROOT returned empty")
	}
	return root, nil
}

const wasmShellHTML = `<!doctype html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, minimum-scale=1, user-scalable=no, viewport-fit=cover">
    <title>%s</title>
    <style>
        html, body {
            margin: 0;
            width: 100%%;
            height: 100%%;
            overflow: hidden;
            overscroll-behavior: none;
            background: %s;
        }

        #game {
            width: 100vw;
            height: 100vh;
            height: 100dvh;
            position: relative;
        }

        #ar-canvas {
            display: block;
            width: 100%%;
            height: 100%%;
            visibility: %s;
            touch-action: none;
            -webkit-tap-highlight-color: transparent;
        }

        #ar-boot {
            position: absolute;
            inset: 0;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: flex-start;
            background: %s;
            --ar-fit-w: min(max(100vw, 80dvh), 177.78dvh);
            --ar-fit-h: calc(var(--ar-fit-w) * 0.5625);
        }

        #ar-poster-frame {
			position: absolute;
            inset: 0;
            width: 100%%;
            height: 100%%;
            z-index: 1;
        }

        #ar-poster {
            display: block;
            width: 100%%;
            height: 100%%;
        }

        #ar-bar {
            position: relative;
            flex: none;
            width: clamp(200px, 60vw, 550px);
            height: clamp(20px, 1.8vw, 40px);
            margin-top: calc(60dvh + 0.155 * var(--ar-fit-h) + 24px);
            border-radius: 4px;
            overflow: hidden;
			z-index: 2;
        }

        @media (max-aspect-ratio: 1/1) {
            #ar-boot {
                --ar-fit-w: min(100vw, 99.6dvh);
                --ar-fit-h: calc(var(--ar-fit-w) * 1.004);
            }

            #ar-bar {
                margin-top: calc(50dvh + 0.444 * var(--ar-fit-h) + 24px);
            }

        }

        #ar-bar {
            background: linear-gradient(to right,
                rgba(224, 234, 255, 0.45) 0%%, rgba(224, 234, 255, 0.45) 25%%,
                rgba(214, 214, 235, 0.62) 25%%, rgba(214, 214, 235, 0.62) 50%%,
                rgba(224, 195, 145, 0.82) 50%%, rgba(224, 195, 145, 0.82) 75%%,
                #E3B156 75%%, #E3B156 100%%);
        }

        #ar-bar::after {
            content: '';
            position: absolute;
            inset: 0;
            pointer-events: none;
            background: repeating-linear-gradient(to right,
                transparent 0, transparent calc(25%% - 1px),
                #020B14 calc(25%% - 1px), #020B14 calc(25%% + 1px));
        }

        #ar-bar-fill {
            position: absolute;
            inset: 0;
            left: 0%%;
            background: rgba(2, 11, 20, 0.88);
            transition: left 0.15s linear;
        }

        #ar-bar-fill::before {
            content: '';
            position: absolute;
            top: -3px;
            left: 0;
            width: 2px;
            height: calc(100%% + 6px);
            background: #F4E4C1;
            box-shadow: 0 0 6px 1px rgba(244, 228, 193, 0.75);
            animation: ar-bar-tick 1.6s ease-in-out infinite;
        }

        @keyframes ar-bar-tick {
            0%%, 100%% { opacity: 0.45; }
            50%%       { opacity: 1; }
        }

        #ar-bar-fill.ar-bar-indeterminate {
            left: 0%% !important;
            width: 40%%;
            background: rgba(216, 183, 122, 0.9);
            animation: ar-bar-sweep 1.1s ease-in-out infinite;
        }

        #ar-bar-fill.ar-bar-indeterminate::before {
            display: none;
        }

        @keyframes ar-bar-sweep {
            0%%   { transform: translateX(-100%%); }
            100%% { transform: translateX(350%%); }
        }
    </style>
</head>
<body>
<div id="game">
    %s
    <canvas id="ar-canvas"></canvas>
</div>

<script src="wasm_exec.js"></script>
<script>
(async () => {
    window.arasyDisableSplash = %t;
    const boot = document.getElementById('ar-boot');
    const fill = document.getElementById('ar-bar-fill');
    const canvas = document.getElementById('ar-canvas');
    const go = new Go();
    function reveal() {
        canvas.style.visibility = 'visible';
        if (boot) boot.remove();
        window.arasyBootFadeDone = true;
    }

    function waitForFirstFrame() {
        if (window.arasyFirstFrame) {
            reveal();
            return;
        }
        requestAnimationFrame(waitForFirstFrame);
    }

    try {
        const resp = await fetch(%s);
        const total = Number(resp.headers.get('X-Arasy-Uncompressed-Length')) ||
            Number(resp.headers.get('Content-Length')) || 0;

        let instantiated;
        if (resp.body && total > 0) {
            const reader = resp.body.getReader();
            const chunks = [];
            let received = 0;
            for (;;) {
                const { done, value } = await reader.read();
                if (done) break;
                chunks.push(value);
                received += value.length;
                if (fill) fill.style.left = Math.min(100, (received / total) * 100) + '%%';
            }
            const bytes = new Uint8Array(received);
            let offset = 0;
            for (const chunk of chunks) {
                bytes.set(chunk, offset);
                offset += chunk.length;
            }
            instantiated = await WebAssembly.instantiate(bytes.buffer, go.importObject);
        } else {
            if (fill) fill.classList.add('ar-bar-indeterminate');
            instantiated = await WebAssembly.instantiateStreaming(resp, go.importObject);
        }

        waitForFirstFrame();
        go.run(instantiated.instance);
    } catch (e) {
        if (boot) {
            boot.textContent = 'Failed to load: ' + e;
        } else {
            document.body.textContent = 'Failed to load: ' + e;
        }
    }
})();
</script>
</body>
</html>
`

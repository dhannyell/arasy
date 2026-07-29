// Package wasmserve create a simple HTTP server to run a compiled wasm file in browser
package wasmserve

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
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

// posterBackground matches the flat backdrop the poster and the compiled
// engine splash both paint, so the page never shows a mismatched edge around
// the inline image.
const posterBackground = "#020B14"

type ServerOptions struct {
	Dir          string
	Title        string
	WASMFile     string
	WASMExecPath string
	Addr         string
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
		Dir:          opts.Dir,
		Title:        opts.Title,
		WASMFile:     opts.WASMFile,
		WASMExecPath: opts.WASMExecPath,
		Rebuild:      opts.Rebuild,
	})

	fs := http.FileServer(http.Dir(opts.Dir))

	log.Info("listening", "addr", opts.Addr, "dir", opts.Dir, "wasm", opts.WASMFile, "wasm_exec", opts.WASMExecPath)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")

		fs.ServeHTTP(w, r)
	})
	if err := http.ListenAndServe(opts.Addr, handler); err != nil {
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

func serveIndex(w http.ResponseWriter, opts ServerOptions) {
	body, err := ShellHTML(opts.Title, opts.WASMFile)
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
	encodedWASMFile, err := json.Marshal(wasmFile)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(wasmShellHTML,
		html.EscapeString(title), posterBackground, posterBackground,
		string(posterSVG), encodedWASMFile,
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
            /* 100dvh tracks the visual viewport (mobile browser chrome
               showing/hiding); 100vh is the fallback for browsers that
               predate the dvh unit. */
            height: 100dvh;
            position: relative;
        }

        #ar-canvas {
            display: block;
            width: 100%%;
            height: 100%%;
            visibility: hidden;
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
            /* Nudges the centered poster+bar group up: justify-content:
               center balances around the padded content box, so a
               bottom-only pad shifts the midpoint (and everything centered
               on it) up by about half of itself, without needing an extra
               wrapper element to transform. */
            background: %s;
        }

        #ar-poster {
            /* Kept inline so its root viewport is the real boot surface:
               ambient particles retain a fixed CSS-pixel size while the
               nested logo layer owns its independent contain projection. */
			position: absolute;
            inset: 0;
            width: 100%%;
            height: 100%%;
            display: block;
            z-index: 1;
        }

        /* The track itself paints 4 fixed bands at a constant position
           regardless of how much has loaded, each one closer to the ring's
           own gold (#D8B77A, see internal/enginesplash's spark) than the
           last — the same cool starlight-to-gold progression the Crux
           stars' spectral tints trace (internal/enginesplash/crux.go), as
           if the phases warm into the mark's own color as they complete.
           #ar-bar-fill is a dark mask covering whatever hasn't loaded yet;
           as real download progress advances, the mask recedes from the
           left and continuously reveals more of this fixed gradient, so
           motion tracks real progress smoothly instead of only jumping at
           each 25%% boundary. */
        #ar-bar {
            position: relative;
            flex: none;
            width: min(110vw, 25vw);
            height: 0.9vw;
            margin-top: 70vh;
            border-radius: 4px;
            overflow: hidden;
			z-index: 2;
            background: linear-gradient(to right,
                rgba(224, 234, 255, 0.45) 0%%, rgba(224, 234, 255, 0.45) 25%%,
                rgba(214, 214, 235, 0.62) 25%%, rgba(214, 214, 235, 0.62) 50%%,
                rgba(224, 195, 145, 0.82) 50%%, rgba(224, 195, 145, 0.82) 75%%,
                #E3B156 75%%, #E3B156 100%%);
        }

        /* Thin fixed dividers between the 4 phases, painted above the mask
           so the reveal visibly crosses through each one rather than
           jumping between hard-edged blocks. */
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

        /* A marker at the mask's own leading edge — it moves only as true
           download progress advances (left tracks the real percentage), and
           only its brightness pulses in place, not its position. */
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

        /* No known total (indeterminate): falls back to a plain sweeping
           highlight instead of the phase reveal above, since there's no real
           fraction to map onto the 4 bands. */
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
    <div id="ar-boot">
        %s

        <div id="ar-bar">
			<div id="ar-bar-fill"></div>
		</div>
    </div>
    <canvas id="ar-canvas"></canvas>
</div>

<script src="wasm_exec.js"></script>
<script>
(async () => {
    const boot = document.getElementById('ar-boot');
    const fill = document.getElementById('ar-bar-fill');
    const canvas = document.getElementById('ar-canvas');
    const go = new Go();
    // reveal hides the poster with a hard cut the instant the live first
    // frame is ready. The revealed segment fades ITSELF in
    // (SplashSegment.Fade) -- the poster does not also cross-fade, so
    // there is only ever one fade running, not two competing ones.
    function reveal() {
        canvas.style.visibility = 'visible';
        boot.remove();
        window.arasyBootFadeDone = true;
    }

    // arasyFirstFrame is the shell <-> host contract: the WASM host sets it
    // once it has presented a frame (splash or game), and only then is the
    // canvas revealed. A host serving app.wasm outside arasy serve/dev/build
    // must set this flag itself to avoid a permanently hidden canvas.
    function waitForFirstFrame() {
        if (window.arasyFirstFrame) {
            reveal();
            return;
        }
        requestAnimationFrame(waitForFirstFrame);
    }

    try {
        const resp = await fetch(%s);
        const total = Number(resp.headers.get('Content-Length')) || 0;

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
                fill.style.left = Math.min(100, (received / total) * 100) + '%%';
            }
            const bytes = new Uint8Array(received);
            let offset = 0;
            for (const chunk of chunks) {
                bytes.set(chunk, offset);
                offset += chunk.length;
            }
            // Buffered instantiation: the tradeoff for a determinate progress
            // bar is losing streaming compilation during download.
            instantiated = await WebAssembly.instantiate(bytes.buffer, go.importObject);
        } else {
            // No Content-Length (e.g. compressed/chunked transfer): an
            // indeterminate sweep is honest about there being no known total,
            // and streaming compilation is kept since nothing needs to be
            // buffered for it.
            fill.classList.add('ar-bar-indeterminate');
            instantiated = await WebAssembly.instantiateStreaming(resp, go.importObject);
        }

        waitForFirstFrame();
        go.run(instantiated.instance);
    } catch (e) {
        boot.textContent = 'Failed to load: ' + e;
    }
})();
</script>
</body>
</html>
`

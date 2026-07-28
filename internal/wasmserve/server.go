// Package wasmserve create a simple HTTP server to run a compiled wasm file in browser
package wasmserve

import (
	_ "embed"
	"encoding/base64"
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

// posterPNG is the engine brand's static boot poster (a still frame of the
// same animation internal/enginesplash compiles), embedded in the CLI binary
// so the shell can paint it inline before any byte of app.wasm has arrived —
// see docs/assets/brand/arasy/splash-lockup/arasy-splash-lockup-poster.png,
// the canonical source this file is copied from.
//
//go:embed assets/poster.png
var posterPNG []byte

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

	log.Info("listening", "addr", opts.Addr, "dir", opts.Dir, "wasm", opts.WASMFile, "wasm_exec", opts.WASMExecPath)
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
	posterDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(posterPNG)
	return []byte(fmt.Sprintf(wasmShellHTML,
		html.EscapeString(title), posterBackground, posterBackground,
		posterDataURI, encodedWASMFile,
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
            justify-content: center;
            background: %s;
        }

        #ar-poster {
            /* The poster is a full frame of the same 1920x1080 composition
               the compiled splash renders — sized to fill the viewport with
               the same letterboxed "contain" fit, not a small centered icon,
               so it reads at the same scale the animation takes over at.
               flex: 1 (with min-height: 0, needed since flex items default
               to min-height: auto) shares the column with #ar-bar below it
               instead of claiming the full column height and pushing the
               bar out of view. */
            flex: 1 1 auto;
            min-height: 0;
            width: 100%%;
            object-fit: contain;
        }

        #ar-bar {
            flex: none;
            width: min(80vw, 320px);
            height: 3px;
            margin-top: 24px;
            background: rgba(255, 255, 255, 0.15);
            border-radius: 2px;
            overflow: hidden;
        }

        #ar-bar-fill {
            width: 0%%;
            height: 100%%;
            background: #D8B77A;
            transition: width 0.15s linear;
        }

        #ar-bar-fill.ar-bar-indeterminate {
            width: 40%% !important;
            animation: ar-bar-sweep 1.1s ease-in-out infinite;
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
        <img id="ar-poster" src="%s" alt="">
        <div id="ar-bar"><div id="ar-bar-fill"></div></div>
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

    // reveal is an atomic handoff: the poster/progress overlay disappears in
    // the same tick the canvas becomes visible, so there is never a frame
    // with both stacked (which would otherwise race screenshot-based tests
    // against a CSS fade).
    function reveal() {
        boot.remove();
        canvas.style.visibility = 'visible';
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
                fill.style.width = Math.min(100, (received / total) * 100) + '%%';
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

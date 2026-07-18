// Package wasmserve create a simple HTTP server to run a compiled wasm file in browser
package wasmserve

import (
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

func Start(opts *ServerOptions) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if opts.WASMExecPath == "" {
		var err error
		opts.WASMExecPath, err = DefaultWASMExecPath()
		if err != nil {
			log.Error("wasm_exec.js not found", "err", err)
			os.Exit(1)
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
		os.Exit(1)
	}
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
// static builds.
func ShellHTML(title, wasmFile string) ([]byte, error) {
	encodedWASMFile, err := json.Marshal(wasmFile)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(wasmShellHTML, html.EscapeString(title), encodedWASMFile)), nil
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
    <title>%s</title>
    <style>
        html, body {
            margin: 0;
            width: 100%%;
            height: 100%%;
            overflow: hidden;
            background: #111;
        }

        #game {
            width: 100vw;
            height: 100vh;
        }

        #ff-canvas {
            display: block;
            width: 100%%;
            height: 100%%;
        }
    </style>
</head>
<body>
<div id="game">
    <canvas id="ff-canvas"></canvas>
</div>

<script src="wasm_exec.js"></script>
<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch(%s), go.importObject)
        .then(r => go.run(r.instance));
</script>
</body>
</html>
`

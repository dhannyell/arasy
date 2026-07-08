package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type serverOptions struct {
	Dir          string
	Title        string
	WASMFile     string
	WASMExecPath string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "HTTP listen address")
	dir := flag.String("dir", "cmd/demos/breakout/wasm/web", "directory to serve")
	title := flag.String("title", "frameforge wasm", "HTML page title")
	wasmFile := flag.String("wasm", "app.wasm", "WASM file served from -dir and loaded by the shell")
	wasmExec := flag.String("wasm-exec", "", "path to wasm_exec.js (default: current Go toolchain)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	wasmExecPath := *wasmExec
	if wasmExecPath == "" {
		var err error
		wasmExecPath, err = defaultWASMExecPath()
		if err != nil {
			log.Error("wasm_exec.js not found", "err", err)
			os.Exit(1)
		}
	}

	handler := newHandler(serverOptions{
		Dir:          *dir,
		Title:        *title,
		WASMFile:     *wasmFile,
		WASMExecPath: wasmExecPath,
	})

	log.Info("listening", "addr", *addr, "dir", *dir, "wasm", *wasmFile, "wasm_exec", wasmExecPath)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func newHandler(opts serverOptions) http.Handler {
	files := http.FileServer(http.Dir(opts.Dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			serveIndex(w, opts)
		case "/wasm_exec.js":
			http.ServeFile(w, r, opts.WASMExecPath)
		default:
			files.ServeHTTP(w, r)
		}
	})
}

func serveIndex(w http.ResponseWriter, opts serverOptions) {
	wasmFile, err := json.Marshal(opts.WASMFile)
	if err != nil {
		http.Error(w, "invalid wasm file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprintf(w, wasmShellHTML, html.EscapeString(opts.Title), wasmFile); err != nil {
		slog.Warn("failed to write index response", "err", err)
	}
}

func defaultWASMExecPath() (string, error) {
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

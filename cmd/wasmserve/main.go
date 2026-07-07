package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	fmt.Fprintf(w, wasmShellHTML, html.EscapeString(opts.Title), wasmFile)
}

func defaultWASMExecPath() (string, error) {
	candidates := []string{
		filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"),
		filepath.Join(runtime.GOROOT(), "misc", "wasm", "wasm_exec.js"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("checked %s", candidates)
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

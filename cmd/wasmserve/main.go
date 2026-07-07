package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "HTTP listen address")
	dir := flag.String("dir", "cmd/demos/breakout/wasm/web", "directory to serve")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	handler := http.FileServer(http.Dir(*dir))

	log.Info("listening", "addr", *addr, "dir", *dir)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

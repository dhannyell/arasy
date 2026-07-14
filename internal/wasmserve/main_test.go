package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerServesGeneratedIndex(t *testing.T) {
	dir := t.TempDir()
	wasmExec := filepath.Join(dir, "wasm_exec.js")
	if err := os.WriteFile(wasmExec, []byte("class Go {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(serverOptions{
		Dir:          dir,
		Title:        "frameforge test",
		WASMFile:     "demo.wasm",
		WASMExecPath: wasmExec,
	})

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>frameforge test</title>") {
		t.Fatalf("index body missing title:\n%s", body)
	}
	if !strings.Contains(body, `fetch("demo.wasm")`) {
		t.Fatalf("index body missing wasm file:\n%s", body)
	}
}

func TestHandlerServesWASMExecFromConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	wasmExec := filepath.Join(dir, "go-wasm.js")
	if err := os.WriteFile(wasmExec, []byte("class Go {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(serverOptions{
		Dir:          dir,
		Title:        "frameforge test",
		WASMFile:     "app.wasm",
		WASMExecPath: wasmExec,
	})

	req := httptest.NewRequest(http.MethodGet, "/wasm_exec.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "class Go {}" {
		t.Fatalf("wasm_exec body = %q", got)
	}
}

func TestHandlerServesArtifactsFromDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	wasmExec := filepath.Join(dir, "wasm_exec.js")
	if err := os.WriteFile(wasmExec, []byte("class Go {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(serverOptions{
		Dir:          dir,
		Title:        "frameforge test",
		WASMFile:     "app.wasm",
		WASMExecPath: wasmExec,
	})

	req := httptest.NewRequest(http.MethodGet, "/app.wasm", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "wasm" {
		t.Fatalf("app.wasm body = %q", got)
	}
}

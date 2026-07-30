package wasmserve

import (
	"compress/gzip"
	"fmt"
	"io"
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

	handler := newHandler(ServerOptions{
		Dir:          dir,
		Title:        "Arasy test",
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
	if !strings.Contains(body, "<title>Arasy test</title>") {
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

	handler := newHandler(ServerOptions{
		Dir:          dir,
		Title:        "Arasy test",
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

func TestHandlerServesResponsivePosterAssets(t *testing.T) {
	handler := newHandler(ServerOptions{})
	for _, name := range []string{PosterLandscapeFile, PosterStackedFile} {
		req := httptest.NewRequest(http.MethodGet, "/"+name, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", name, rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
			t.Fatalf("%s content type = %q", name, got)
		}
		if !strings.Contains(rec.Body.String(), "<svg") {
			t.Fatalf("%s body is not SVG", name)
		}
	}
}

func TestGzipHandlerCompressesSVGWhenAccepted(t *testing.T) {
	handler := gzipHandler(newHandler(ServerOptions{}))
	req := httptest.NewRequest(http.MethodGet, "/"+PosterLandscapeFile, nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("content encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "<svg") {
		t.Fatal("decoded gzip response is not SVG")
	}
}

func TestGzipHandlerLeavesResponsePlainWithoutNegotiation(t *testing.T) {
	handler := gzipHandler(newHandler(ServerOptions{}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+PosterLandscapeFile, nil))

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("content encoding = %q without Accept-Encoding", got)
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatal("plain response is not SVG")
	}
}

func TestGzipHandlerHonorsExplicitZeroQuality(t *testing.T) {
	handler := gzipHandler(newHandler(ServerOptions{}))
	req := httptest.NewRequest(http.MethodGet, "/"+PosterLandscapeFile, nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0, identity")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("content encoding = %q for gzip;q=0", got)
	}
}

func TestGzipWASMPreservesDecodedLengthForDownloadProgress(t *testing.T) {
	dir := t.TempDir()
	const payload = "0123456789abcdef"
	if err := os.WriteFile(filepath.Join(dir, "app.wasm"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := gzipHandler(newHandler(ServerOptions{Dir: dir, WASMFile: "app.wasm"}))
	req := httptest.NewRequest(http.MethodGet, "/app.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(uncompressedLengthHeader); got != fmt.Sprint(len(payload)) {
		t.Fatalf("%s = %q, want %d", uncompressedLengthHeader, got, len(payload))
	}
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Fatalf("compressed Content-Length = %q, want empty streaming length", got)
	}
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if string(body) != payload {
		t.Fatalf("decoded body = %q, want %q", body, payload)
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

	handler := newHandler(ServerOptions{
		Dir:          dir,
		Title:        "Arasy test",
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

func TestHandlerRebuildErrorIsReturnedToBrowser(t *testing.T) {
	dir := t.TempDir()
	wasmExec := filepath.Join(dir, "wasm_exec.js")
	if err := os.WriteFile(wasmExec, []byte("class Go {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := newHandler(ServerOptions{
		Dir: dir, WASMFile: "app.wasm", WASMExecPath: wasmExec,
		Rebuild: func() error { return fmt.Errorf("compile failed") },
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.wasm", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "compile failed") {
		t.Fatalf("status/body = %d/%q, want 500 and compile error", rec.Code, rec.Body.String())
	}
}

func TestShellHTMLEscapesTitleAndWASMFile(t *testing.T) {
	body, err := ShellHTML("<unsafe>", `demo".wasm`)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "&lt;unsafe&gt;") || !strings.Contains(got, `fetch("demo\".wasm")`) {
		t.Fatalf("unexpected shell: %s", got)
	}
}

func TestShellCanDisableLoadingAndRuntimeSplashes(t *testing.T) {
	body, err := shellHTML("Arasy", "app.wasm", true)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	if strings.Contains(html, `id="ar-boot"`) ||
		strings.Contains(html, PosterLandscapeFile) ||
		strings.Contains(html, PosterStackedFile) {
		t.Fatal("disabled shell still contains loading splash markup")
	}
	if !strings.Contains(html, "window.arasyDisableSplash = true;") {
		t.Fatal("disabled shell does not signal runtime splash suppression")
	}
	if !strings.Contains(html, "visibility: visible;") {
		t.Fatal("disabled shell does not expose canvas immediately")
	}
}

func TestShellUsesDecodedLengthHeaderForGzipProgress(t *testing.T) {
	body, err := ShellHTML("Arasy", "app.wasm")
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	headerIndex := strings.Index(html, "resp.headers.get('X-Arasy-Uncompressed-Length')")
	fallbackIndex := strings.Index(html, "resp.headers.get('Content-Length')")
	if headerIndex < 0 || fallbackIndex < 0 || headerIndex > fallbackIndex {
		t.Fatal("shell does not prefer decoded gzip length before Content-Length")
	}
}

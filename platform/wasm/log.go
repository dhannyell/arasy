//go:build js && wasm

package wasm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"syscall/js"
	"time"
)

// NewConsoleLogger returns a slog.Logger that writes directly to the browser
// console and respects the provided minimum level.
func NewConsoleLogger(level slog.Leveler) *slog.Logger {
	return slog.New(consoleHandler{level: level})
}

type consoleHandler struct {
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

func (h consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.level != nil {
		min = h.level.Level()
	}
	return level >= min
}

func (h consoleHandler) Handle(_ context.Context, record slog.Record) error {
	parts := []string{
		record.Time.Format(time.RFC3339),
		record.Level.String(),
		record.Message,
	}

	for _, attr := range h.attrs {
		parts = append(parts, h.formatAttr(attr))
	}
	record.Attrs(func(attr slog.Attr) bool {
		parts = append(parts, h.formatAttr(attr))
		return true
	})

	console := js.Global().Get("console")
	method := consoleMethod(record.Level)
	if !console.Get(method).Truthy() {
		method = "log"
	}
	console.Call(method, strings.Join(parts, " "))
	return nil
}

func (h consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := h
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return next
}

func (h consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := h
	next.groups = append(append([]string(nil), h.groups...), name)
	return next
}

func (h consoleHandler) formatAttr(attr slog.Attr) string {
	attr.Value = attr.Value.Resolve()
	key := attr.Key
	if len(h.groups) > 0 {
		key = strings.Join(append(append([]string(nil), h.groups...), key), ".")
	}
	return fmt.Sprintf("%s=%v", key, attr.Value.Any())
}

func consoleMethod(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return "debug"
	case level < slog.LevelWarn:
		return "info"
	case level < slog.LevelError:
		return "warn"
	default:
		return "error"
	}
}

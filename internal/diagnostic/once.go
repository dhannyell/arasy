// Package diagnostic provides small, process-local helpers for engine
// diagnostics. It intentionally owns no global state: each host keeps its
// own Once value, so one run cannot suppress messages from another.
package diagnostic

import (
	"context"
	"log/slog"
)

// Key identifies one diagnostic occurrence class. Code describes the
// condition and ID distinguishes affected resources within that condition.
type Key struct {
	Code string
	ID   uint64
}

// Once emits each enabled diagnostic key at most once. Its zero value is
// ready to use. Once is intended for the engine's single-goroutine renderer
// paths and must not be used concurrently.
type Once struct {
	seen map[Key]struct{}
}

// Log emits msg at level the first time key is seen. Disabled messages do not
// consume their key. A nil logger uses slog.Default. Arguments are evaluated
// by Go before Log is called; use LogLazy when constructing them is expensive.
func (o *Once) Log(logger *slog.Logger, level slog.Level, key Key, msg string, args ...any) {
	logger, ctx, ok := o.prepare(logger, level, key)
	if !ok {
		return
	}
	o.mark(key)
	logger.Log(ctx, level, msg, args...)
}

// LogLazy emits msg at level the first time key is seen and calls args only
// after confirming that the level is enabled and the key has not been emitted.
// A nil args callback is equivalent to an empty argument list.
func (o *Once) LogLazy(
	logger *slog.Logger,
	level slog.Level,
	key Key,
	msg string,
	args func() []any,
) {
	logger, ctx, ok := o.prepare(logger, level, key)
	if !ok {
		return
	}
	var values []any
	if args != nil {
		values = args()
	}
	o.mark(key)
	logger.Log(ctx, level, msg, values...)
}

func (o *Once) prepare(logger *slog.Logger, level slog.Level, key Key) (*slog.Logger, context.Context, bool) {
	if logger == nil {
		logger = slog.Default()
	}
	ctx := context.Background()
	if !logger.Enabled(ctx, level) {
		return logger, ctx, false
	}
	if _, logged := o.seen[key]; logged {
		return logger, ctx, false
	}
	return logger, ctx, true
}

func (o *Once) mark(key Key) {
	if o.seen == nil {
		o.seen = make(map[Key]struct{})
	}
	o.seen[key] = struct{}{}
}

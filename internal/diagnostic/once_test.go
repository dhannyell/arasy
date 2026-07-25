package diagnostic_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/internal/diagnostic"
)

func TestOnceLogDeduplicatesByCodeAndID(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	var diagnostics diagnostic.Once

	diagnostics.Log(logger, slog.LevelWarn, diagnostic.Key{Code: "missing", ID: 7}, "first")
	diagnostics.Log(logger, slog.LevelWarn, diagnostic.Key{Code: "missing", ID: 7}, "duplicate")
	diagnostics.Log(logger, slog.LevelWarn, diagnostic.Key{Code: "missing", ID: 8}, "different id")
	diagnostics.Log(logger, slog.LevelWarn, diagnostic.Key{Code: "invalid", ID: 7}, "different code")

	logs := output.String()
	if got := strings.Count(logs, "level=WARN"); got != 3 {
		t.Fatalf("warning count = %d, want 3; logs:\n%s", got, logs)
	}
	if strings.Contains(logs, "duplicate") {
		t.Fatalf("duplicate diagnostic was emitted; logs:\n%s", logs)
	}
}

func TestOnceLogDoesNotConsumeDisabledKey(t *testing.T) {
	var output bytes.Buffer
	var diagnostics diagnostic.Once
	key := diagnostic.Key{Code: "optional", ID: 1}

	warnLogger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn}))
	diagnostics.Log(warnLogger, slog.LevelInfo, key, "disabled")

	infoLogger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	diagnostics.Log(infoLogger, slog.LevelInfo, key, "enabled later")

	logs := output.String()
	if !strings.Contains(logs, "enabled later") || strings.Contains(logs, "disabled") {
		t.Fatalf("disabled key was consumed or emitted; logs:\n%s", logs)
	}
}

func TestOnceLogLazyBuildsArgumentsOnlyWhenEnabledAndUnseen(t *testing.T) {
	var output bytes.Buffer
	var diagnostics diagnostic.Once
	key := diagnostic.Key{Code: "expensive", ID: 1}
	builds := 0
	build := func() []any {
		builds++
		return []any{"checksum", 42}
	}

	warnLogger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn}))
	diagnostics.LogLazy(warnLogger, slog.LevelDebug, key, "disabled", build)
	if builds != 0 {
		t.Fatalf("argument builder calls after disabled log = %d, want 0", builds)
	}

	debugLogger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	diagnostics.LogLazy(debugLogger, slog.LevelDebug, key, "enabled", build)
	diagnostics.LogLazy(debugLogger, slog.LevelDebug, key, "duplicate", build)

	if builds != 1 {
		t.Fatalf("argument builder calls = %d, want 1", builds)
	}
	logs := output.String()
	if !strings.Contains(logs, "checksum=42") || strings.Contains(logs, "duplicate") {
		t.Fatalf("lazy diagnostic output is wrong; logs:\n%s", logs)
	}
}

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dhannyell/frameforge/cmd/demos/breakout"
	"github.com/dhannyell/frameforge/cmd/demos/breakout/audioassets"
	"github.com/dhannyell/frameforge/platform/web/canvas"
	"github.com/dhannyell/frameforge/runtime"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	replayIn := flag.String("replay-in", "", "optional .ffrp recording to verify and replay before serving")
	recordOut := flag.String("record-out", "", "optional .ffrp path to write when the server exits")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if *replayIn != "" && *recordOut != "" {
		log.Error("replay-in and record-out cannot be used together")
		os.Exit(1)
	}
	driver := runtime.NewDriver(&breakout.Breakout{}, log)

	if *replayIn != "" {
		file, err := os.Open(*replayIn)
		if err != nil {
			log.Error("open replay failed", "path", *replayIn, "err", err)
			os.Exit(1)
		}
		rec, err := runtime.ReadRecording(file)
		closeErr := file.Close()
		if err != nil {
			log.Error("read replay failed", "path", *replayIn, "err", err)
			os.Exit(1)
		}
		if closeErr != nil {
			log.Error("close replay failed", "path", *replayIn, "err", closeErr)
			os.Exit(1)
		}
		if err := runtime.VerifyRecording(&breakout.Breakout{}, log, rec); err != nil {
			log.Error("verify replay failed", "path", *replayIn, "err", err)
			os.Exit(1)
		}
		driver = runtime.Replay(&breakout.Breakout{}, log, rec)
		log.Info("replay loaded", "path", *replayIn, "ticks", rec.Ticks, "events", len(rec.Events), "anchors", len(rec.Anchors))
	}

	if *recordOut != "" {
		driver.StartRecording()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := canvas.RunDriver(ctx, driver, canvas.Options{Addr: *addr, Log: log, AudioFS: audioassets.FS()})
	if *recordOut != "" {
		if writeErr := writeRecording(*recordOut, driver.StopRecording()); writeErr != nil {
			log.Error("write recording failed", "path", *recordOut, "err", writeErr)
			os.Exit(1)
		}
		log.Info("recording saved", "path", *recordOut)
	}
	if err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func writeRecording(path string, rec runtime.Recording) error {
	data, err := rec.MarshalBinary()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

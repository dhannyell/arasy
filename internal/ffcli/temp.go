package ffcli

import (
	"os"
	"os/signal"
)

func removeTempDir(dir string) error {
	if dir == "" {
		return nil
	}
	return os.RemoveAll(dir)
}

func watchTempDirInterrupt(dir string) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case <-signals:
			_ = removeTempDir(dir)
			os.Exit(130)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

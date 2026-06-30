package ui

import (
	"hash/fnv"
	"log/slog"
)

type ID uint64

const noID ID = 0

func HashID(s string) ID {
	h := fnv.New64a()
	_, err := h.Write([]byte(s))
	if err != nil {
		slog.Error("Failed hashing ID")
		return 0
	}
	return ID(h.Sum64())
}

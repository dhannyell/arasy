package runtime

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// timeline owns the rewindable journal independently from the fixed-step
// driver loop. Events and viewport changes are kept in tick order; edits made
// while rewound remain staged until the next simulation step forks the log.
type timeline struct {
	events    []input.Event
	viewports []viewportEntry
	staged    []input.Event

	stagedViewport    math2d.Rect
	hasStagedViewport bool
	maxTick           uint64
}

func (t *timeline) record(tick uint64, events []input.Event) {
	for _, event := range events {
		event.Tick = tick
		if tick < t.maxTick {
			t.staged = append(t.staged, event)
		} else {
			t.events = append(t.events, event)
		}
	}
}

func (t *timeline) recordViewport(tick uint64, viewport math2d.Rect) {
	if tick < t.maxTick {
		if t.hasStagedViewport && t.stagedViewport == viewport {
			return
		}
		t.stagedViewport = viewport
		t.hasStagedViewport = true
		return
	}
	t.viewports = appendViewportLogEntry(t.viewports, tick, viewport)
}

// fork commits staged changes at tick and discards the old future.
func (t *timeline) fork(tick uint64, viewport math2d.Rect) {
	t.events = truncateAtTick(t.events, tick)
	t.events = append(t.events, t.staged...)
	t.staged = nil

	t.viewports = truncateViewportLogAtTick(t.viewports, tick)
	if t.hasStagedViewport {
		t.viewports = appendViewportLogEntry(t.viewports, tick, t.stagedViewport)
		t.stagedViewport = math2d.Rect{}
		t.hasStagedViewport = false
	} else {
		t.viewports = appendViewportLogEntry(t.viewports, tick, viewport)
	}
	t.maxTick = tick
}

func (t *timeline) cursorFrom(tick uint64) resimulationCursor {
	return newResimulationCursor(t.events, t.viewports, tick)
}

func (t *timeline) viewportAt(tick uint64, fallback math2d.Rect) math2d.Rect {
	viewport := fallback
	for _, entry := range t.viewports {
		if entry.tick > tick {
			break
		}
		viewport = entry.viewport
	}
	return viewport
}

// advanceTo raises maxTick to tick if tick is further ahead, and is a no-op
// otherwise. Used by stepOnce after a normal (non-rewound) step.
func (t *timeline) advanceTo(tick uint64) {
	if tick > t.maxTick {
		t.maxTick = tick
	}
}

func (t *timeline) discardStaged() {
	t.staged = nil
	t.stagedViewport = math2d.Rect{}
	t.hasStagedViewport = false
}

func (t *timeline) reset() {
	t.events = nil
	t.viewports = nil
	t.maxTick = 0
	t.discardStaged()
}

func truncateAtTick(events []input.Event, tick uint64) []input.Event {
	kept := events[:0]
	for _, event := range events {
		if event.Tick < tick {
			kept = append(kept, event)
		}
	}
	return kept
}

// truncateViewportLogAtTick drops viewport entries at or after the fork tick,
// matching truncateAtTick for input events.
func truncateViewportLogAtTick(log []viewportEntry, tick uint64) []viewportEntry {
	kept := log[:0]
	for _, entry := range log {
		if entry.tick < tick {
			kept = append(kept, entry)
		}
	}
	return kept
}

func appendViewportLogEntry(log []viewportEntry, tick uint64, viewport math2d.Rect) []viewportEntry {
	if viewport.Width() <= 0 || viewport.Height() <= 0 {
		return log
	}
	if len(log) > 0 && log[len(log)-1].viewport == viewport {
		return log
	}
	return append(log, viewportEntry{tick: tick, viewport: viewport})
}

type resimulationCursor struct {
	events        []input.Event
	eventIndex    int
	viewportLog   []viewportEntry
	viewportIndex int
}

func newResimulationCursor(events []input.Event, viewportLog []viewportEntry, tick uint64) resimulationCursor {
	cursor := resimulationCursor{events: events, viewportLog: viewportLog}
	for cursor.eventIndex < len(cursor.events) && cursor.events[cursor.eventIndex].Tick < tick {
		cursor.eventIndex++
	}
	for cursor.viewportIndex < len(cursor.viewportLog) && cursor.viewportLog[cursor.viewportIndex].tick < tick {
		cursor.viewportIndex++
	}
	return cursor
}

func (c *resimulationCursor) eventsAt(tick uint64) []input.Event {
	for c.eventIndex < len(c.events) && c.events[c.eventIndex].Tick < tick {
		c.eventIndex++
	}
	start := c.eventIndex
	for c.eventIndex < len(c.events) && c.events[c.eventIndex].Tick == tick {
		c.eventIndex++
	}
	return c.events[start:c.eventIndex]
}

func (c *resimulationCursor) viewportsAt(tick uint64) []math2d.Rect {
	for c.viewportIndex < len(c.viewportLog) && c.viewportLog[c.viewportIndex].tick < tick {
		c.viewportIndex++
	}
	start := c.viewportIndex
	for c.viewportIndex < len(c.viewportLog) && c.viewportLog[c.viewportIndex].tick == tick {
		c.viewportIndex++
	}
	entries := c.viewportLog[start:c.viewportIndex]
	viewports := make([]math2d.Rect, 0, len(entries))
	for _, entry := range entries {
		viewports = append(viewports, entry.viewport)
	}
	return viewports
}

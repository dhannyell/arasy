package devpanel

import (
	"strconv"
	"time"
)

func formatMillis(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds()*1000, 'f', 1, 64) + "ms"
}

func formatMegabytes(bytes uint64) string {
	return strconv.FormatFloat(float64(bytes)/(1024*1024), 'f', 1, 64) + "MB"
}

func formatGigabytes(bytes uint64) string {
	return strconv.FormatFloat(float64(bytes)/(1024*1024*1024), 'f', 1, 64) + "GB"
}

func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64) + "%"
}

func formatUint(v uint64) string { return strconv.FormatUint(v, 10) }

func formatInt(v int) string { return strconv.Itoa(v) }

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func formatBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// parseTick reads a tick typed into the panel. Anything that is not a plain
// non-negative number is rejected rather than coerced — seeking to a tick the
// user did not mean is worse than doing nothing.
func parseTick(s string) (uint64, bool) {
	tick, err := strconv.ParseUint(s, 10, 64)
	return tick, err == nil
}

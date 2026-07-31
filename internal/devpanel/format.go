package devpanel

import "strconv"

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

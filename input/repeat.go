package input

// Default repeat timing, used when a Repeater's fields are zero. The values
// are the classic menu-cursor feel: a beat before repeating, then brisk.
const (
	DefaultRepeatDelay    = 0.35
	DefaultRepeatInterval = 0.09
)

// Repeater turns a held input into evenly spaced steps: one step the tick it
// is pressed, then after Delay a step every Interval while it stays held. It
// is the clock the input frames deliberately do not run -- KeyFrame and the
// Mapper report edges without auto-repeat, so that repeating identically live
// and in replay is the consumer's dt-driven loop, which is this type.
//
// Every field is exported so a Repeater can live inside snapshotted game
// state; the zero value is ready to use with the default timing.
type Repeater struct {
	Delay    float64
	Interval float64

	Held  bool
	Timer float64
}

// Step advances the clock by dt with the input held or not, and returns how
// many steps fire this tick -- usually 0 or 1, more when a long dt crosses
// several intervals, so a hitch cannot silently swallow steps.
func (r *Repeater) Step(held bool, dt float64) int {
	if !held {
		r.Held = false
		return 0
	}

	delay, interval := r.Delay, r.Interval
	if delay <= 0 {
		delay = DefaultRepeatDelay
	}
	if interval <= 0 {
		interval = DefaultRepeatInterval
	}

	if !r.Held {
		r.Held = true
		r.Timer = delay
		return 1
	}

	steps := 0
	r.Timer -= dt
	for r.Timer <= 0 {
		steps++
		r.Timer += interval
	}
	return steps
}

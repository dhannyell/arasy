//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import "time"

// framePacer caps the loop to a target frame budget. It aims at an absolute
// deadline so a transient oversleep does not accumulate into long-run drift,
// and optionally busy-spins the final spinMargin for tighter pacing at the cost
// of one core. A zero budget disables pacing (uncapped).
type framePacer struct {
	budget     time.Duration // 0 = uncapped
	spinMargin time.Duration // 0 = pure sleep; e.g. 1ms = hybrid sleep+spin
	next       time.Time     // absolute deadline for the frame in flight
}

// wait blocks until the current frame's deadline, then advances the schedule.
// Call it once at the end of every loop iteration.
func (p *framePacer) wait() {
	if p.budget <= 0 {
		return
	}
	now := time.Now()
	if p.next.IsZero() {
		p.next = now
	}
	p.next = p.next.Add(p.budget)

	// Catch-up guard: after a stall (resize, breakpoint, GC spike) the deadline
	// can fall far behind. Abandon the backlog and re-anchor to now instead of
	// racing through frames with no sleep to "catch up".
	if now.Sub(p.next) > p.budget {
		p.next = now.Add(p.budget)
		return
	}

	// Sleep the bulk, keeping spinMargin in hand. On Go >=1.23 the Windows
	// sleep uses a high-resolution waitable timer (~0.5ms granularity).
	if sleepUntil := p.next.Add(-p.spinMargin); now.Before(sleepUntil) {
		time.Sleep(time.Until(sleepUntil))
	}

	// Busy-spin the final margin for sub-millisecond precision. A no-op when
	// spinMargin is zero (pure-sleep mode). Tight spin (no Gosched) keeps the
	// wake precise; it burns one core for at most spinMargin.
	for time.Now().Before(p.next) {
	}
}

// frameBudget is the minimum wall-clock duration between rendered frames for
// the configured FPS cap. A cap of zero or below means uncapped (zero is
// normalized away by the driver; a negative cap is an explicit request).
func frameBudget(fps int) time.Duration {
	if fps <= 0 {
		return 0
	}
	return time.Second / time.Duration(fps)
}

// pacerBudget is the budget for the software pacer given the presentation
// mode. With VSync on it is zero — the swapchain is already the clock, and a
// second pacer at the same nominal rate only fights it: the real refresh
// never equals the nominal budget exactly (59.94Hz, 60.03Hz), so whichever
// is slower periodically forces a frame to wait a full extra interval.
// Only with VSync off does the pacer become the clock.
func pacerBudget(fps int, vsync VSyncMode) time.Duration {
	if vsync == VSyncOn {
		return 0
	}
	return frameBudget(fps)
}

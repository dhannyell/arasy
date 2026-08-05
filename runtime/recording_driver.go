package runtime

import "log/slog"

// newDriverFromRecording is the single reconstruction path for seed and
// deterministic input policy. A zero or invalid in-memory recording value
// leaves the fresh game's effective deadzone (or simulation rate) in place; a
// valid explicit value overrides the driver's config.
func newDriverFromRecording(game Game, log *slog.Logger, rec Recording) *Driver {
	driver := newDriver(game, log, &rec.Seed, &rec.SimulationHz)
	if rec.GamepadDeadzone > 0 && rec.GamepadDeadzone < 1 {
		driver.cfg.GamepadDeadzone = rec.GamepadDeadzone
		driver.input.SetGamepadDeadzone(rec.GamepadDeadzone)
	}
	return driver
}

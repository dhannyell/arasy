//go:build arr_dev

package runtime

import "github.com/dhannyell/arasy/scene"

// The accessors below open the driver's insides to development tooling — the
// dev panel's inspector reads the world and the game state through them, and
// writes components back through DebugWorld.
//
// They exist only in builds tagged arr_dev, the same tag that compiles the
// panel in, and they are deliberately not part of the engine's compatibility
// promise: a game that calls one of them will not build in release. The tag
// is the line between tooling and shipped code, and this is tooling.

// WorldView returns the read-only face of the simulation world — the same
// view games receive through the update/render contexts. The inspector uses
// it to enumerate entities and read their components; mutation through it is
// a compile error by construction.
func (d *Driver) WorldView() scene.WorldView { return d.world.View() }

// DebugWorld exposes the live simulation world for development tooling that
// mutates state on purpose — the inspector's field editor. Mutations land
// between ticks exactly like input does, but they are not journaled: a
// session edited this way no longer replays exactly, which the recording
// verification will catch. Games never see this; they get the world through
// the update context.
//
// Handing the world out is the moment to assume it is about to change, so
// the overlay's per-tick checksum memo is dropped here: a stats line still
// showing the pre-edit checksum would be reporting a world that no longer
// exists.
func (d *Driver) DebugWorld() *scene.World {
	d.debugChecksumValid = false
	return d.world
}

// DebugGameState captures a fresh snapshot of the game's state and returns
// it unwrapped from its GameState box. ok is false when the game does not
// implement StatefulGame. The value is a snapshot — read it, never mutate it.
func (d *Driver) DebugGameState() (value any, ok bool) {
	state := d.captureGameState()
	if state == nil {
		return nil, false
	}
	return DebugStateValue(state), true
}

// DebugStateValue unwraps the value a GameState boxes. The value shares the
// snapshot's ownership: read it, never mutate it. Nil state yields nil.
func DebugStateValue(state GameState) any {
	if state == nil {
		return nil
	}
	return state.debugValue()
}

// Package middle sits between the game and the simulation root. The test
// uses it to verify the transitive import walk.
package middle

import "simroot"

// Step forwards to the simulation root so the import remains reachable.
func Step() { simroot.Tick() }

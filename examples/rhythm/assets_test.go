package rhythm_test

import (
	"testing"

	"github.com/dhannyell/frameforge/examples/rhythm"
)

func TestNewGameLoadsDeclaredAssets(t *testing.T) {
	t.Parallel()

	if _, err := rhythm.NewGame(); err != nil {
		t.Fatalf("NewGame() error = %v", err)
	}
}

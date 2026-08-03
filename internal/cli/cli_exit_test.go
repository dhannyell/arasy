package cli

import (
	"errors"
	"fmt"
	"testing"
)

type fakeExitError struct{ code int }

func (e fakeExitError) Error() string { return fmt.Sprintf("exit %d", e.code) }
func (e fakeExitError) ExitCode() int { return e.code }

func TestExitCodeForResolvesTheStatusAnErrorNames(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
	}{
		"plain error":  {err: errors.New("boom"), want: 1},
		"named status": {err: fakeExitError{code: 3}, want: 3},
		"wrapped":      {err: fmt.Errorf("run game: %w", fakeExitError{code: 42}), want: 42},
		// A game that names 0 still failed to run, or we would not be here.
		"names zero": {err: fakeExitError{code: 0}, want: 1},
	}
	for name, tc := range cases {
		if got := exitCodeFor(tc.err); got != tc.want {
			t.Fatalf("%s: exitCodeFor = %d, want %d", name, got, tc.want)
		}
	}
}

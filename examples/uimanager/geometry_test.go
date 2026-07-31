package uimanager

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

// The right-hand cards are derived by cutting a single strip instead of being
// written out as three literals. These are the literals they replaced, kept
// here so the derivation cannot drift off the tuned layout the goldens show.
func TestTablePanelsKeepTheirHandTunedGeometry(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  math2d.Rect
		want math2d.Rect
	}{
		{"round", tableRoundPanel(), rect(970, 170, 770, 200)},
		{"next match", tableNextMatchPanel(), rect(970, 390, 770, 300)},
		{"legend", tableLegendPanel(), rect(970, 710, 770, 150)},
	} {
		if tc.got != tc.want {
			t.Errorf("%s panel = %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
}

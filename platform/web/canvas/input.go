package canvas

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/platform/web/browserinput"
)

func BrowserKeyboardCode(code string) (input.SourceCode, bool) {
	return browserinput.KeyboardCode(code)
}

package browserinput_test

import (
	"testing"

	"github.com/dhannyell/arasy/platform/web/browserinput"
)

func TestSingleTouchIgnoresSecondPointerWithoutChangingOwner(t *testing.T) {
	var touch browserinput.SingleTouch

	if !touch.Accept(11, browserinput.TouchBegin) {
		t.Fatal("first touch begin rejected")
	}
	if touch.Accept(22, browserinput.TouchBegin) {
		t.Fatal("second touch begin accepted")
	}
	if touch.Accept(22, browserinput.TouchMove) {
		t.Fatal("second touch move accepted")
	}
	if touch.Accept(22, browserinput.TouchEnd) {
		t.Fatal("second touch end accepted")
	}
	if !touch.Accept(11, browserinput.TouchMove) {
		t.Fatal("first touch lost ownership after second touch ended")
	}
	if !touch.Accept(11, browserinput.TouchEnd) {
		t.Fatal("first touch end rejected")
	}
}

func TestSingleTouchAcceptsNextPointerAfterOwnerEnds(t *testing.T) {
	var touch browserinput.SingleTouch

	if !touch.Accept(0, browserinput.TouchBegin) {
		t.Fatal("pointer ID zero begin rejected")
	}
	if !touch.Accept(0, browserinput.TouchEnd) {
		t.Fatal("pointer ID zero end rejected")
	}
	if !touch.Accept(22, browserinput.TouchBegin) {
		t.Fatal("next touch begin rejected")
	}
}

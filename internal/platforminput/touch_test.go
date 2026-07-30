package platforminput_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/platforminput"
)

func TestSingleTouchIgnoresSecondPointerWithoutChangingOwner(t *testing.T) {
	var touch platforminput.SingleTouch

	if !touch.Accept(11, platforminput.TouchBegin) {
		t.Fatal("first touch begin rejected")
	}
	if touch.Accept(22, platforminput.TouchBegin) {
		t.Fatal("second touch begin accepted")
	}
	if touch.Accept(22, platforminput.TouchMove) {
		t.Fatal("second touch move accepted")
	}
	if touch.Accept(22, platforminput.TouchEnd) {
		t.Fatal("second touch end accepted")
	}
	if !touch.Accept(11, platforminput.TouchMove) {
		t.Fatal("first touch lost ownership after second touch ended")
	}
	if !touch.Accept(11, platforminput.TouchEnd) {
		t.Fatal("first touch end rejected")
	}
}

func TestSingleTouchAcceptsNextPointerAfterOwnerEnds(t *testing.T) {
	var touch platforminput.SingleTouch

	if !touch.Accept(0, platforminput.TouchBegin) {
		t.Fatal("pointer ID zero begin rejected")
	}
	if !touch.Accept(0, platforminput.TouchEnd) {
		t.Fatal("pointer ID zero end rejected")
	}
	if !touch.Accept(22, platforminput.TouchBegin) {
		t.Fatal("next touch begin rejected")
	}
}

package ui

import "testing"

func TestDefaultThemeProvidesManagerSlots(t *testing.T) {
	theme := DefaultTheme()

	if theme.Card.CornerRadius <= 0 {
		t.Fatal("card corner radius should have a default")
	}
	if theme.Row.Height <= 0 {
		t.Fatal("row height should have a default")
	}
	if theme.Header.Height <= 0 {
		t.Fatal("header height should have a default")
	}
	if theme.Badge.TextSize <= 0 {
		t.Fatal("badge text size should have a default")
	}
	if theme.Text.Body <= 0 || theme.Text.Caption <= 0 {
		t.Fatal("text scale should have defaults")
	}
	if theme.Focus.Width <= 0 {
		t.Fatal("focus width should have a default")
	}
}

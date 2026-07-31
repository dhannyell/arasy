package ui

import (
	"math"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// TextInput is caller-owned, like SliderInput and the caller-owned Tabs: the
// game passes the text it holds and stores the text it gets back. Typed text
// is something the game means — a search box filters the game's data, a rename
// changes the game's world — so it belongs in game state, where it rewinds with
// the simulation and shows up in a replay fingerprint. What the widget keeps
// for itself is the caret position and the repeat clock, which nobody would
// think to declare and which live in ui.State, cloned with the rest of it.
//
// Editing is driven by the tick's input.KeyFrame: characters arrive as
// EventText and editing keys as ordinary key buttons, so a recorded session
// replays a typed word exactly. Within one tick, characters are inserted
// before editing keys act.

const (
	// textRepeatDelay and textRepeatRate drive auto-repeat for editing keys
	// from ui.State instead of from the OS: the widget counts ticks a key has
	// been held, so a held backspace deletes at the same rate live and in a
	// replay, on any platform. At 60Hz these are ~0.5s before the first
	// repeat and ~20 deletions a second after it.
	textRepeatDelay = 30
	textRepeatRate  = 3
)

// TextInputOptions configures a text field. The zero value is a plain,
// unlimited field with no placeholder.
type TextInputOptions struct {
	// Placeholder is drawn in the muted colour while the text is empty.
	Placeholder string
	// MaxRunes caps the length in characters (not bytes). Zero is unlimited.
	MaxRunes int
}

// TextInputResult is what one tick of editing produced. Text is the value to
// store; Changed says whether it differs from what was passed in; Submitted
// reports an Enter press while the field held focus. Submit rides in the
// result rather than in an out-parameter so the whole outcome of the tick
// arrives in one value, and adding to it later breaks nobody.
type TextInputResult struct {
	Text      string
	Changed   bool
	Submitted bool
}

// textInputIn is called in Update. It resolves focus from this tick's pointer
// events, applies the tick's typed characters and editing keys, and returns
// the resulting text.
func (u *UI) textInputIn(id string, rect math2d.Rect, text string, opts TextInputOptions, space Space) TextInputResult {
	wid := HashID(id)
	u.focusableIn(id, rect, space)
	u.ensureCaret()

	runes := []rune(text)
	caret := clampInt(u.state.caret[wid], 0, len(runes))

	caret = u.textInputFocus(wid, rect, caret, runes, space)

	result := TextInputResult{Text: text}
	if u.state.focusID == wid {
		runes, caret, result.Submitted = u.editText(runes, caret, opts)
		next := string(runes)
		result.Text = next
		result.Changed = next != text
	}

	u.state.caret[wid] = clampInt(caret, 0, len(runes))
	return result
}

// textInputFocus resolves clicks: a press inside the field focuses it and puts
// the caret where the pointer landed, a press anywhere else gives up focus, so
// a field stops eating keys the moment the player looks elsewhere.
func (u *UI) textInputFocus(wid ID, rect math2d.Rect, caret int, runes []rune, space Space) int {
	for _, e := range u.frame.Events {
		if e.Button != input.PointerPrimary || e.Kind != input.EventButtonDown {
			continue
		}
		if u.blockedAt(wid, e.Pos) {
			continue
		}
		pos := u.cameraFor(space).ScreenToWorld(e.Pos)
		if rect.Contains(pos) {
			u.state.focusID = wid
			caret = caretAtX(runes, caret, pos.X, rect, u.theme.TextInput)
			continue
		}
		if u.state.focusID == wid {
			u.state.focusID = noID
		}
	}
	return caret
}

// editText applies the tick's characters and editing keys to the caret.
func (u *UI) editText(runes []rune, caret int, opts TextInputOptions) ([]rune, int, bool) {
	for _, r := range u.keys.Text {
		if opts.MaxRunes > 0 && len(runes) >= opts.MaxRunes {
			break
		}
		runes = insertRune(runes, caret, r)
		caret++
	}

	acts := u.keyActs
	if acts(input.KeyBackspace) && caret > 0 {
		runes = append(runes[:caret-1], runes[caret:]...)
		caret--
	}
	if acts(input.KeyDelete) && caret < len(runes) {
		runes = append(runes[:caret], runes[caret+1:]...)
	}
	if acts(input.KeyArrowLeft) {
		caret--
	}
	if acts(input.KeyArrowRight) {
		caret++
	}
	if acts(input.KeyHome) {
		caret = 0
	}
	if acts(input.KeyEnd) {
		caret = len(runes)
	}

	return runes, clampInt(caret, 0, len(runes)), u.keys.JustPressed(input.KeyEnter)
}

// textInputDrawIn draws the field, its text and — while focused — the caret.
// Like every draw in this package it only reads state: the horizontal scroll
// is derived from the caret rather than stored, so nothing here can drift from
// what Update decided.
func (u *UI) textInputDrawIn(id string, rect math2d.Rect, text string, opts TextInputOptions, space Space) {
	theme := u.theme.TextInput
	if rect.Width() <= 0 || rect.Height() <= 0 {
		return
	}

	wid := HashID(id)
	focused := u.state.focusID == wid
	runes := []rune(text)
	caret := clampInt(u.state.caret[wid], 0, len(runes))

	background := theme.Background
	border := theme.Border
	if focused {
		background = theme.BackgroundFocused
		border = u.theme.Focus.Color
	} else if u.hoveredIn(rect, space) {
		background = theme.BackgroundHovered
	}

	layer := u.nextLayer()
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(rect, space),
		Fill:         background,
		Stroke:       border,
		StrokeWidth:  u.sizeToWorld(theme.BorderWidth, space),
		CornerRadius: u.sizeToWorld(theme.CornerRadius, space),
	})

	inner := textInputInnerRect(rect, theme)
	clip := u.rectToWorld(inner, space)
	offset := textInputScroll(runes, caret, inner, theme)

	body, color := text, theme.TextColor
	if body == "" {
		body, color = opts.Placeholder, theme.PlaceholderColor
	}
	if body != "" {
		u.commands = append(u.commands, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1, Clip: clip},
			Position:    u.toWorld(math2d.Vec2{X: inner.Min.X - offset, Y: rect.Center().Y}, space),
			Text:        body,
			Color:       color,
			Size:        u.sizeToWorld(theme.TextSize, space),
			Font:        u.theme.Font,
			Align:       render.AlignLeft,
			Baseline:    render.BaselineMiddle,
		})
	}

	if !focused || !caretVisible(u.time, theme.BlinkPeriod) {
		return
	}
	caretX := inner.Min.X + textWidth(runes[:caret], theme.TextSize) - offset
	caretRect := math2d.NewRect(
		math2d.Vec2{X: caretX, Y: inner.Min.Y},
		math2d.Vec2{X: math.Max(theme.CaretWidth, 1), Y: inner.Height()},
	)
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer + 2, Clip: clip},
		Rect:        u.rectToWorld(caretRect, space),
		Fill:        theme.CaretColor,
	})
}

// caretVisible blinks off the render clock, never off simulation state: a
// caret that blinked in the world would change the checksum and make a replay
// depend on how long a frame took.
func caretVisible(time, period float64) bool {
	if period <= 0 {
		return true
	}
	return math.Mod(time, period) < period/2
}

func textInputInnerRect(rect math2d.Rect, theme TextInputTheme) math2d.Rect {
	pad := math.Min(theme.PaddingX, rect.Width()/2)
	return math2d.NewRect(
		math2d.Vec2{X: rect.Min.X + pad, Y: rect.Min.Y + theme.PaddingY},
		math2d.Vec2{X: math.Max(0, rect.Width()-2*pad), Y: math.Max(0, rect.Height()-2*theme.PaddingY)},
	)
}

// textInputScroll returns how far the text is shifted left so the caret stays
// inside the field. It only shifts as far as the caret demands, which keeps a
// short value pinned to the left edge.
func textInputScroll(runes []rune, caret int, inner math2d.Rect, theme TextInputTheme) float64 {
	caretX := textWidth(runes[:caret], theme.TextSize)
	overflow := caretX - inner.Width() + math.Max(theme.CaretWidth, 1)
	return math.Max(0, overflow)
}

// caretAtX maps a pointer x to the nearest character boundary. It reproduces
// the shift the draw derived from the caret it is replacing, so clicking lands
// where the player saw the character, not where an unscrolled field would have
// put it.
func caretAtX(runes []rune, caret int, x float64, rect math2d.Rect, theme TextInputTheme) int {
	inner := textInputInnerRect(rect, theme)
	advance := render.MeasureText("n", theme.TextSize).X
	if advance <= 0 {
		return len(runes)
	}
	offset := x - inner.Min.X + textInputScroll(runes, clampInt(caret, 0, len(runes)), inner, theme)
	return clampInt(int(math.Round(offset/advance)), 0, len(runes))
}

func textWidth(runes []rune, size float64) float64 {
	if len(runes) == 0 {
		return 0
	}
	return render.MeasureText(string(runes), size).X
}

func insertRune(runes []rune, at int, r rune) []rune {
	at = clampInt(at, 0, len(runes))
	runes = append(runes, 0)
	copy(runes[at+1:], runes[at:])
	runes[at] = r
	return runes
}

func (u *UI) ensureCaret() {
	if u.state.caret == nil {
		u.state.caret = make(map[ID]int)
	}
}

func clampInt(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

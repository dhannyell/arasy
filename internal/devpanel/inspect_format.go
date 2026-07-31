//go:build arr_dev

package devpanel

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/dhannyell/arasy/render"
)

// Limits keep the inspector bounded no matter what a game state holds: a
// cyclic pointer chain stops at the line cap, a huge slice at the item cap,
// a deep nest at the depth cap. This is a debug view — a truncated answer is
// more useful than a hung frame.
const (
	inspectMaxDepth = 5
	inspectMaxItems = 8
	inspectMaxLines = 512
)

// formatTree renders a value as indented lines, one field or item per line,
// for the inspector's scrollable list. Structs whose fields are all scalars
// (Vec2, Color, ...) stay on one line; everything else expands.
func formatTree(root any) []string {
	f := &treeFormatter{lines: make([]string, 0, 32)}
	f.walk("", reflect.ValueOf(root), 0)
	return f.lines
}

type treeFormatter struct {
	lines []string
}

func (f *treeFormatter) emit(depth int, s string) {
	if len(f.lines) >= inspectMaxLines {
		if f.lines[len(f.lines)-1] != "…" {
			f.lines = append(f.lines, "…")
		}
		return
	}
	f.lines = append(f.lines, strings.Repeat("  ", depth)+s)
}

// walk appends the lines for one named value. The root call passes an empty
// name: a struct root lists its fields without a redundant header line.
func (f *treeFormatter) walk(name string, v reflect.Value, depth int) {
	if !v.IsValid() {
		f.emit(depth, name+": nil")
		return
	}
	if depth > inspectMaxDepth {
		f.emit(depth, name+": …")
		return
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			f.emit(depth, name+": nil")
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		// The nameless root always expands field by field — a one-line dump
		// of the whole game state is exactly what the tab exists to avoid.
		if name != "" && inlineStruct(v) {
			f.emit(depth, fmt.Sprintf("%s: %+v", name, v))
			return
		}
		if name != "" {
			f.emit(depth, name+": "+v.Type().String())
		}
		// A named struct indents its fields under the header line; the
		// nameless root lists its fields at the top level.
		fieldDepth := depth + 1
		if name == "" {
			fieldDepth = depth
		}
		for i := 0; i < v.NumField(); i++ {
			f.walk(v.Type().Field(i).Name, v.Field(i), fieldDepth)
		}
	case reflect.Slice, reflect.Array:
		f.emit(depth, fmt.Sprintf("%s: %s [len %d]", name, v.Type(), v.Len()))
		shown := min(v.Len(), inspectMaxItems)
		for i := range shown {
			f.walk(fmt.Sprintf("[%d]", i), v.Index(i), depth+1)
		}
		if rest := v.Len() - shown; rest > 0 {
			f.emit(depth+1, fmt.Sprintf("… +%d more", rest))
		}
	case reflect.Map:
		f.emit(depth, fmt.Sprintf("%s: %s [len %d]", name, v.Type(), v.Len()))
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int {
			return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
		})
		shown := min(len(keys), inspectMaxItems)
		for i := range shown {
			f.walk(fmt.Sprintf("[%v]", keys[i]), v.MapIndex(keys[i]), depth+1)
		}
		if rest := len(keys) - shown; rest > 0 {
			f.emit(depth+1, fmt.Sprintf("… +%d more", rest))
		}
	default:
		// fmt reads through reflect.Value, including unexported fields —
		// the caller can only format this tree, never mutate it.
		f.emit(depth, fmt.Sprintf("%s: %v", name, v))
	}
}

// inlineStruct reports whether a struct is scalar enough to print on one
// line: every field is a plain value (no nesting, no collections).
func inlineStruct(v reflect.Value) bool {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		switch t.Field(i).Type.Kind() {
		case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map,
			reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan:
			return false
		}
	}
	return true
}

// truncateToWidth shortens s with an ellipsis so it measures at most width,
// keeping list rows inside the panel instead of bleeding past the edge.
func truncateToWidth(s string, width, size float64) string {
	if render.MeasureText(s, size).X <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 1 {
		runes = runes[:len(runes)-1]
		if candidate := string(runes) + "…"; render.MeasureText(candidate, size).X <= width {
			return candidate
		}
	}
	return "…"
}

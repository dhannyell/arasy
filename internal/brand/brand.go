// Package brand holds the engine's own artwork for the demos that ship with
// it. It is the icon an Arasy demo wears, not a default the engine imposes: a
// game supplies its own image through the host's Icon option, and the icon a
// file manager shows for the executable is a separate, per-game resource
// built with go-winres (see docs/en/distribution.md).
//
// The source of truth for the brand art lives outside the module, in a
// directory that is not versioned. The image here is a checked-in copy,
// because an //go:embed of an unversioned path builds on the machine that has
// it and nowhere else.
package brand

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

// A window icon is drawn small — a title bar, a task switcher entry — so the
// checked-in copy is the 64-pixel size rather than the largest one available.
//
//go:embed assets/appicon-64.png
var appIconPNG []byte

var (
	appIconOnce sync.Once
	appIcon     image.Image
)

// AppIcon returns the engine's window icon. Decoding happens once; callers
// share the image and must not modify it.
func AppIcon() image.Image {
	appIconOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(appIconPNG))
		if err != nil {
			// The bytes are compiled in and never change at runtime, so a
			// failure here is a broken build, not a runtime condition.
			panic("brand: decode embedded app icon: " + err.Error())
		}
		appIcon = img
	})
	return appIcon
}

package enginesplash

import (
	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/internal/enginesplash/stacked"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Content retains the generated landscape and portrait workspaces.
type Content struct {
	landscape ArasySplashWorkspace
	portrait  stacked.ArasySplashStackedWorkspace
}

// Append renders the responsive engine splash. Returned commands remain valid
// until the next call on the same Content.
func (c *Content) Append(timeline anim.Timeline, viewport math2d.Rect) []render.Command {
	if viewport.Width() < viewport.Height() {
		return c.portrait.Append(
			timeline,
			stackedViewport(viewport),
			stacked.ArasySplashStackedResources{},
		)
	}
	return c.landscape.Append(
		timeline,
		engineSplashViewport(viewport),
		ArasySplashResources{},
	)
}

package devpanel

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

// pickEntity finds the topmost entity whose collider contains worldPos.
// Topmost follows the render order (Layer, Y, EntityID): the entity the user
// sees on top is the one the click should land on. Entities without a
// collider are not pickable — the same shapes the debug layers draw are the
// shapes the click sees, and mirroring the collision code (raw radius and
// half extents at the transform position, rotation and scale ignored) keeps
// the two in agreement.
func pickEntity(view scene.WorldView, entities []scene.EntityID, worldPos math2d.Vec2) (scene.EntityID, bool) {
	var best scene.EntityID
	bestLayer := 0
	bestY := 0.0
	found := false

	for _, id := range entities {
		if !colliderContains(view, id, worldPos) {
			continue
		}
		transform, _ := view.Transform(id)
		layer := spriteLayer(view, id)
		if !found ||
			layer > bestLayer ||
			(layer == bestLayer && transform.Position.Y > bestY) ||
			(layer == bestLayer && transform.Position.Y == bestY && id > best) {
			best, bestLayer, bestY, found = id, layer, transform.Position.Y, true
		}
	}
	return best, found
}

func colliderContains(view scene.WorldView, id scene.EntityID, point math2d.Vec2) bool {
	transform, ok := view.Transform(id)
	if !ok {
		return false
	}
	dx := point.X - transform.Position.X
	dy := point.Y - transform.Position.Y

	if circle, ok := view.CircleCollider(id); ok {
		return float64(dx*dx)+float64(dy*dy) <= circle.Radius*circle.Radius
	}
	if rect, ok := view.RectCollider(id); ok {
		return abs64(dx) <= rect.HalfExtends.X && abs64(dy) <= rect.HalfExtends.Y
	}
	return false
}

// spriteLayer is the render layer of an entity's first sprite, the same
// Layer the depth sort uses; zero when the entity draws nothing.
func spriteLayer(view scene.WorldView, id scene.EntityID) int {
	if sprite, ok := view.CircleSprite(id); ok {
		return sprite.Layer
	}
	if sprite, ok := view.RectSprite(id); ok {
		return sprite.Layer
	}
	if sprite, ok := view.ImageSprite(id); ok {
		return sprite.Layer
	}
	return 0
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

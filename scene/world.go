package scene

import "github.com/dhannyell/frameforge/render"

type World struct {
	nextID EntityID

	Transforms    map[EntityID]Transform
	CircleSprites map[EntityID]CircleSprite
}

func NewWorld() *World {
	return &World{
		nextID:        1,
		Transforms:    make(map[EntityID]Transform),
		CircleSprites: make(map[EntityID]CircleSprite),
	}
}

func (w *World) Spawn() EntityID {
	id := w.nextID
	w.nextID++

	return id
}

func (w *World) Despawn(id EntityID) {
	delete(w.Transforms, id)
	delete(w.CircleSprites, id)
}

func (w *World) SetTransform(id EntityID, transform Transform) {
	w.Transforms[id] = transform
}

func (w *World) SetCircleSprite(id EntityID, sprite CircleSprite) {
	w.CircleSprites[id] = sprite
}

func (w *World) Transform(id EntityID) (Transform, bool) {
	transform, ok := w.Transforms[id]
	return transform, ok
}

func (w *World) CircleSprite(id EntityID) (CircleSprite, bool) {
	sprite, ok := w.CircleSprites[id]
	return sprite, ok
}

func (w *World) RemoveTransform(id EntityID) {
	delete(w.Transforms, id)
}

func (w *World) RemoveCircleSprite(id EntityID) {
	delete(w.CircleSprites, id)
}

func (w *World) RenderCommands() []render.Command {
	commands := make([]render.Command, 0, len(w.CircleSprites))

	for id, sprite := range w.CircleSprites {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		commands = append(commands, render.DrawCircle{
			Center: transform.Position,
			Radius: sprite.Radius,
			Fill:   sprite.Color,
			Layer:  sprite.Layer,
		})
	}

	return commands
}

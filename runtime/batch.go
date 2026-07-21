package runtime

import (
	"slices"
	"sort"

	"github.com/dhannyell/frameforge/render"
)

// BatchOp identifies the operation of a retained-batch update: replace the
// whole content or delete the resource. Content is never significant for a
// delete, and nil content never means delete.
type BatchOp uint8

const (
	// BatchReplace replaces the whole content of a retained batch. An empty
	// slice creates or keeps an empty batch; it is not a delete.
	BatchReplace BatchOp = iota
	// BatchDelete removes the retained batch and releases its resources.
	BatchDelete
)

// SpriteBatchUpdate carries one retained sprite batch operation drained from
// the driver. Instances is meaningful only for BatchReplace; the slice is
// owned by the consumer.
type SpriteBatchUpdate struct {
	ID        render.SpriteBatchID
	Op        BatchOp
	Instances []render.SpriteInstance
}

// TextBatchUpdate carries one retained text batch operation drained from the
// driver. Labels is meaningful only for BatchReplace; the slice is owned by
// the consumer.
type TextBatchUpdate struct {
	ID     render.TextBatchID
	Op     BatchOp
	Labels []render.TextLabel
}

// driverBatches owns the producer side of retained-batch updates. This is
// render-only state: never snapshotted, never recorded; rewind neither drops
// nor undoes publications. Updates are keyed by ID so multiple operations on
// the same batch before the drain collapse to the last one.
type driverBatches struct {
	sprites map[render.SpriteBatchID]SpriteBatchUpdate
	text    map[render.TextBatchID]TextBatchUpdate
}

func (b *driverBatches) replaceSprites(id render.SpriteBatchID, instances []render.SpriteInstance) {
	if b.sprites == nil {
		b.sprites = make(map[render.SpriteBatchID]SpriteBatchUpdate)
	}
	b.sprites[id] = SpriteBatchUpdate{ID: id, Op: BatchReplace, Instances: instances}
}

func (b *driverBatches) deleteSprites(id render.SpriteBatchID) {
	if b.sprites == nil {
		b.sprites = make(map[render.SpriteBatchID]SpriteBatchUpdate)
	}
	b.sprites[id] = SpriteBatchUpdate{ID: id, Op: BatchDelete}
}

func (b *driverBatches) replaceText(id render.TextBatchID, labels []render.TextLabel) {
	if b.text == nil {
		b.text = make(map[render.TextBatchID]TextBatchUpdate)
	}
	b.text[id] = TextBatchUpdate{ID: id, Op: BatchReplace, Labels: labels}
}

func (b *driverBatches) deleteText(id render.TextBatchID) {
	if b.text == nil {
		b.text = make(map[render.TextBatchID]TextBatchUpdate)
	}
	b.text[id] = TextBatchUpdate{ID: id, Op: BatchDelete}
}

// consume returns the pending updates sorted by batch ID for deterministic
// ordering and zeroes the pending set.
func (b *driverBatches) consume() (sprites []SpriteBatchUpdate, text []TextBatchUpdate) {
	if len(b.sprites) > 0 {
		sprites = make([]SpriteBatchUpdate, 0, len(b.sprites))
		for _, u := range b.sprites {
			sprites = append(sprites, u)
		}
		sort.Slice(sprites, func(i, j int) bool { return sprites[i].ID < sprites[j].ID })
	}
	if len(b.text) > 0 {
		text = make([]TextBatchUpdate, 0, len(b.text))
		for _, u := range b.text {
			text = append(text, u)
		}
		sort.Slice(text, func(i, j int) bool { return text[i].ID < text[j].ID })
	}
	b.sprites = nil
	b.text = nil
	return sprites, text
}

func (b *driverBatches) reset() {
	b.sprites = nil
	b.text = nil
}

// BatchPort is the game-facing retained-batch API, shared by UpdateContext
// (Game.Init/Game.Update) and RenderContext (Game.Render). Published content
// is copied before the call returns, so the game may reuse its slices.
// Content is render-only: it never enters snapshots, checksums, replays or
// recordings, and rewind never undoes a publication. Multiple operations on
// the same ID before the frame drain collapse to the last one; an empty
// replace keeps an empty batch — only the delete methods remove a batch.
type BatchPort struct {
	batches *driverBatches
}

// UpdateSprites replaces the whole content of a retained sprite batch.
func (p *BatchPort) UpdateSprites(id render.SpriteBatchID, instances []render.SpriteInstance) {
	if p == nil || p.batches == nil {
		return
	}
	p.batches.replaceSprites(id, slices.Clone(instances))
}

// DeleteSprites removes a retained sprite batch and releases its resources.
func (p *BatchPort) DeleteSprites(id render.SpriteBatchID) {
	if p == nil || p.batches == nil {
		return
	}
	p.batches.deleteSprites(id)
}

// UpdateText replaces the whole content of a retained text batch.
func (p *BatchPort) UpdateText(id render.TextBatchID, labels []render.TextLabel) {
	if p == nil || p.batches == nil {
		return
	}
	p.batches.replaceText(id, slices.Clone(labels))
}

// DeleteText removes a retained text batch and releases its resources.
func (p *BatchPort) DeleteText(id render.TextBatchID) {
	if p == nil || p.batches == nil {
		return
	}
	p.batches.deleteText(id)
}

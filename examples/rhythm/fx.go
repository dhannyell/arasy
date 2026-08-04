package rhythm

import (
	"image"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// Fire and lightning are cosmetic effects. Fire advances in Update using
// private state; bolts derive from render time, the current tick, and
// MatchState. Neither effect can change gameplay state or checksums.

const (
	fireLayer      = 2
	lightningLayer = 13

	// lightningFlashMS is the brief lightning window after a combo start or
	// successful special note.
	lightningFlashMS = 110
	boltSegments     = 10
	boltTopY         = spawnY + 2
)

// hash01 maps a seed to a deterministic value in [0, 1).
func hash01(seed uint64) float64 {
	seed ^= seed >> 33
	seed *= 0xff51afd7ed558ccd
	seed ^= seed >> 33
	seed *= 0xc4ceb9fe1a85ec53
	seed ^= seed >> 33
	return float64(seed>>11) / float64(1<<53)
}

// Fire is a classic heat-propagation simulation (Doom PSX style) rendered
// into the fire_sim dynamic texture and drawn as flame bands on both sides
// of the highway.
const (
	fireW      = 96
	fireH      = 128
	fireStepHz = 30
	// The low-combo fire is a continuous band under the highway, rather than
	// two side strips with a gap in the middle.
	fireBaseWidth  = worldW
	fireBaseHeight = 18.0
	fireMaxCombo   = 100
	fireTallCombo  = 80

	// Fire bolts are short, occasional arcs that become more frequent and
	// taller with the combo. They stay inside the flame bands.
	fireBoltMinCombo = 10
	fireBoltSegments = 5
)

type fireSim struct {
	heat       [fireW * fireH]uint8
	source     [fireW]uint8  // slow-evolving bottom-row heat, forms the tongues
	edge       [fireW]uint16 // per-column fade so the inner band edge has no seam
	palette    [256][4]uint8 // premultiplied RGBA
	accum      float64
	rng        uint64
	styleCombo int

	// Dirty marks that the rendered image can differ from the last publish
	// (a step advanced the heat field, or setCombo rebuilt the palette/heat).
	// This is render-only cosmetic bookkeeping, same as the rest of fireSim:
	// it never enters snapshots, checksums, or recordings.
	dirty bool
	// Buffers double-buffers the published image: UpdateTexture documents that
	// ownership of the *image.RGBA transfers to the engine, so a single
	// reusable image cannot be safely rewritten in place after publishing.
	// Alternating between two preallocated images avoids allocating on every
	// dirty frame while still respecting that ownership transfer.
	buffers [2]*image.RGBA
	next    int
}

func newFireSim() *fireSim {
	f := &fireSim{rng: 0x053f1a2b3c4d5e6f}
	for i := range f.palette {
		f.palette[i] = firePaletteEntry(float64(i)/255, 0)
	}
	for x := range f.source {
		f.source[x] = uint8(190 + f.rand()%66)
	}
	for x := range f.edge {
		f.edge[x] = 256
	}
	f.buffers[0] = image.NewRGBA(image.Rect(0, 0, fireW, fireH))
	f.buffers[1] = image.NewRGBA(image.Rect(0, 0, fireW, fireH))
	// Pre-run so flames are fully developed on the first visible frame.
	for range fireH {
		f.step()
	}
	// The sim always starts dirty so the first frame publishes, regardless of
	// the pre-run above (which also marks dirty via step).
	f.dirty = true
	return f
}

// fireStyle maps the current combo into the render-only flame treatment. A
// zero combo intentionally returns the minimum fire size and light palette.
func fireStyle(combo int) (scale, colorShift float64) {
	if combo < 0 {
		combo = 0
	}
	colorShift = math.Min(float64(combo)/fireMaxCombo, 1)
	// Ease out the height so high, achievable combos visibly charge the
	// stage: combo 85 already reaches 97.75% of the maximum height.
	heightProgress := colorShift * (2 - colorShift)
	// At the maximum combo the flame is anchored at the bottom and reaches
	// Y=0, the top of the rhythm world's 75-unit stage.
	return 1 + heightProgress*(worldH/fireBaseHeight-1), colorShift
}

// firePaletteEntry maps normalized heat to a premultiplied RGBA color. Low
// combo starts in the stage's light crimson/red family and progresses to
// saturated yellow as the combo charges, retaining the heat detail.
func firePaletteEntry(t, colorShift float64) [4]uint8 {
	a := math.Min(math.Max((t-0.06)*2.2, 0), 1)
	lightR, lightG, lightB := 0.72+0.28*t, 0.08+0.20*t, 0.17+0.35*t
	yellowR, yellowG, yellowB := 1.0, 0.42+0.55*t, 0.02+0.25*t
	r := lightR + (yellowR-lightR)*colorShift
	g := lightG + (yellowG-lightG)*colorShift
	b := lightB + (yellowB-lightB)*colorShift
	return [4]uint8{
		uint8(r * a * 255),
		uint8(g * a * 255),
		uint8(b * a * 255),
		uint8(a * 255),
	}
}

// setCombo refreshes the palette only when the visible combo changes.
// MatchState remains the sole gameplay source of truth.
func (f *fireSim) setCombo(combo int) {
	if combo < 0 {
		combo = 0
	}
	if combo > fireMaxCombo {
		combo = fireMaxCombo
	}
	if combo == f.styleCombo {
		return
	}
	previousCombo := f.styleCombo
	f.styleCombo = combo
	_, colorShift := fireStyle(combo)
	for i := range f.palette {
		f.palette[i] = firePaletteEntry(float64(i)/255, colorShift)
	}
	f.dirty = true
	// Rebuild the heat field when entering/leaving the tall-flame range.
	// Without this catch-up, the 30 Hz cellular simulation would need several
	// seconds to carry heat through the newly visible top of the texture.
	if combo == 0 && previousCombo > 0 {
		for i := range f.heat {
			f.heat[i] = 0
		}
		f.settle(fireH)
	} else if combo >= fireTallCombo && previousCombo < fireTallCombo {
		f.settle(fireH)
	}
}

// update advances the cosmetic simulation from the fixed-timestep Update
// path. Render can therefore be called repeatedly without consuming random
// values or changing the image it is about to draw.
func (f *fireSim) update(dt float64, combo int) {
	f.setCombo(combo)
	f.accum += dt
	for f.accum >= 1.0/fireStepHz {
		f.accum -= 1.0 / fireStepHz
		f.step()
	}
}

func (f *fireSim) settle(steps int) {
	for range steps {
		f.step()
	}
}

func (f *fireSim) rand() uint64 {
	f.rng ^= f.rng << 13
	f.rng ^= f.rng >> 7
	f.rng ^= f.rng << 17
	return f.rng
}

// step lifts heat one row up using a weighted average of the three cells
// below plus lateral wind jitter and mild random cooling, then evolves the
// bottom-row source so tongues stay coherent instead of per-pixel noise.
func (f *fireSim) step() {
	f.dirty = true
	for y := 1; y < fireH; y++ {
		row := y * fireW
		for x := range fireW {
			left, right := x-1, x+1
			if left < 0 {
				left = 0
			}
			if right >= fireW {
				right = fireW - 1
			}
			sum := int(f.heat[row+left]) + 2*int(f.heat[row+x]) + int(f.heat[row+right])
			heat := sum / 4
			r := f.rand()
			heat -= f.cooling(r)
			if heat < 0 {
				heat = 0
			}
			dx := x + int(r%3) - 1
			if dx < 0 {
				dx = 0
			} else if dx >= fireW {
				dx = fireW - 1
			}
			f.heat[(y-1)*fireW+dx] = uint8(heat)
		}
	}

	// Random-walk the source row, then relax it toward its neighbors so
	// tongues stay several pixels wide and drift instead of flickering.
	for x := range f.source {
		r := f.rand()
		v := int(f.source[x]) + int(r%25) - 12
		if r%37 == 0 {
			v -= 70
		}
		if v < 130 {
			v = 130
		} else if v > 255 {
			v = 255
		}
		f.source[x] = uint8(v)
	}
	bottom := (fireH - 1) * fireW
	for x := range fireW {
		left, right := x-1, x+1
		if left < 0 {
			left = 0
		}
		if right >= fireW {
			right = fireW - 1
		}
		f.heat[bottom+x] = uint8((int(f.source[left]) + 2*int(f.source[x]) + int(f.source[right])) / 4)
	}
}

func (f *fireSim) cooling(random uint64) int {
	cooling := 2 + int((random>>2)&1) // original cooling: 2..3
	if f.styleCombo >= 40 {
		cooling--
	}
	if f.styleCombo >= fireTallCombo {
		cooling--
	}
	if cooling < 1 {
		return 1
	}
	return cooling
}

// Image renders the current heat field into the next free double-buffer
// slot and alternates it for next time. A fresh buffer is required each call
// because UpdateTexture takes ownership of the returned image; reusing a
// single buffer in place would risk mutating an image the engine still holds.
func (f *fireSim) image() *image.RGBA {
	img := f.buffers[f.next]
	f.next = (f.next + 1) % len(f.buffers)
	for i, heat := range f.heat {
		c := f.palette[heat]
		k := f.edge[i%fireW]
		img.Pix[i*4+0] = uint8(uint16(c[0]) * k / 256)
		img.Pix[i*4+1] = uint8(uint16(c[1]) * k / 256)
		img.Pix[i*4+2] = uint8(uint16(c[2]) * k / 256)
		img.Pix[i*4+3] = uint8(uint16(c[3]) * k / 256)
	}
	return img
}

// consumeDirty reports whether the rendered image changed since the last
// call and clears the flag. drawFire uses this to skip republishing the fire
// texture on frames where the 30 Hz simulation hasn't stepped and the combo
// palette hasn't changed.
func (f *fireSim) consumeDirty() bool {
	dirty := f.dirty
	f.dirty = false
	return dirty
}

// drawFire publishes the current fire image (only when it changed since the
// last frame) and draws it behind the highway. The effect is advanced by
// Update, so this function does not mutate Game.
func (g *Game) drawFire(ctx *runtime.RenderContext, combo int) []render.Command {
	f := g.fire
	texID := ctx.Assets.Texture("fire_sim")
	if f.consumeDirty() {
		ctx.UpdateTexture(texID, f.image())
	}

	scale, _ := fireStyle(combo)
	width := fireBaseWidth
	height := fireBaseHeight * scale
	top := worldH - height
	cmds := []render.Command{
		render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: fireLayer},
			Texture:     texID,
			Dest:        math2d.NewRect(math2d.Vec2{X: 0, Y: top}, math2d.Vec2{X: width, Y: height}),
		},
	}
	return append(cmds, fireBoltCommands(combo, ctx.Time, width, height)...)
}

// fireBoltCommands draws intermittent electric arcs in the middle of the
// growing flame bands. Render time is intentional here: this is cosmetic and
// must not influence simulation, recording, or replay state.
func fireBoltCommands(combo int, renderTime, width, height float64) []render.Command {
	if combo < fireBoltMinCombo {
		return nil
	}
	_, intensity := fireStyle(combo)
	interval := 1.35 - intensity*0.65
	duration := 0.045 + intensity*0.105
	cycle := math.Floor(renderTime / interval)
	within := renderTime - cycle*interval
	if within < 0 || within > duration {
		return nil
	}
	pulse := 1 - within/duration
	seed := uint64(cycle)*0x9e3779b97f4a7c15 + uint64(combo)*0xc2b2ae3d27d4eb4f
	boltHeight := height * (0.18 + intensity*0.38)
	centerY := worldH - height*0.5
	fromY := centerY + boltHeight*0.5
	toY := centerY - boltHeight*0.5

	x := width * (0.18 + hash01(seed)*0.64)
	cmds := fireBoltStroke(fireBoltPath(x, fromY, toY, width, seed), pulse, intensity)
	// At high combo, a second arc spreads the charge across the full fire
	// band without turning every pulse into permanent lightning.
	if intensity >= 0.65 {
		secondX := width * (0.18 + hash01(seed^0x517cc1b727220a95)*0.64)
		cmds = append(cmds, fireBoltStroke(fireBoltPath(secondX, fromY, toY, width, seed^0x517cc1b727220a95), pulse, intensity)...)
	}
	return cmds
}

func fireBoltPath(centerX, fromY, toY, width float64, seed uint64) []math2d.Vec2 {
	points := make([]math2d.Vec2, 0, fireBoltSegments+1)
	for segment := 0; segment <= fireBoltSegments; segment++ {
		t := float64(segment) / fireBoltSegments
		x := centerX
		if segment != 0 && segment != fireBoltSegments {
			x += (hash01(seed+uint64(segment)*17) - 0.5) * width * 0.18
		}
		points = append(points, math2d.Vec2{X: x, Y: fromY + (toY-fromY)*t})
	}
	return points
}

func fireBoltStroke(points []math2d.Vec2, pulse, intensity float64) []render.Command {
	cmds := make([]render.Command, 0, (len(points)-1)*2)
	for i := 0; i+1 < len(points); i++ {
		cmds = append(cmds,
			render.DrawLine{
				BaseCommand: render.BaseCommand{LayerValue: fireLayer},
				From:        points[i], To: points[i+1], Width: 0.72 + intensity*0.28,
				Color: render.RGBA(0.25, 0.58, 1, (0.24+intensity*0.20)*pulse),
			},
			render.DrawLine{
				BaseCommand: render.BaseCommand{LayerValue: fireLayer},
				From:        points[i], To: points[i+1], Width: 0.20 + intensity*0.10,
				Color: render.RGBA(0.92, 0.98, 1, (0.65+intensity*0.25)*pulse),
			},
		)
	}
	return cmds
}

// drawLightning renders a bolt only for meaningful gameplay events: a new
// combo or a successful special note. Ordinary key presses never trigger it.
func drawLightning(ctx *runtime.RenderContext, state MatchState, nowMS float64) []render.Command {
	if !state.LightningActive || state.LightningLane < 0 || state.LightningLane >= laneCount {
		return nil
	}
	remaining := float64(state.LightningUntilMS) - nowMS
	if remaining < 0 || remaining > lightningFlashMS {
		return nil
	}
	k := float64(remaining) / lightningFlashMS
	seed := (uint64(state.LightningLane)+1)*0x9e3779b97f4a7c15 + (ctx.Tick/2)*0x2545f4914f6cdd1d
	return boltCommands(state.LightningLane, k, seed)
}

func boltCommands(lane int, k float64, seed uint64) []render.Command {
	points := boltPath(lane, seed, boltTopY, hitLineY, boltSegments, 1)
	cmds := make([]render.Command, 0, boltSegments*2+8)
	cmds = append(cmds, boltStroke(points, k, 1)...)

	// One short branch forks sideways from the middle of the main bolt.
	forkAt := boltSegments / 2
	branchSeed := seed ^ 0xd6e8feb86659fd93
	branch := boltPath(lane, branchSeed, points[forkAt].Y, points[forkAt].Y+8, 2, 1.8)
	branch[0] = points[forkAt]
	cmds = append(cmds, boltStroke(branch, k*0.6, 0.55)...)
	return cmds
}

// boltPath walks from topY to bottomY pinning both endpoints to the lane
// center and jittering the interior points inside the lane bounds. spread
// widens the jitter beyond the lane for branches.
func boltPath(lane int, seed uint64, topY, bottomY float64, segments int, spread float64) []math2d.Vec2 {
	points := make([]math2d.Vec2, 0, segments+1)
	for s := 0; s <= segments; s++ {
		t := float64(s) / float64(segments)
		y := topY + (bottomY-topY)*t
		left, right := laneBoundsAtY(lane, y, 0.18)
		center := (left + right) / 2
		jitter := 0.0
		if s != 0 && s != segments {
			jitter = (hash01(seed+uint64(s)*11) - 0.5) * (right - left) * spread
		}
		points = append(points, math2d.Vec2{X: center + jitter, Y: y})
	}
	return points
}

// boltStroke draws a wide dim glow pass under a thin bright core pass, with
// widths following the highway perspective.
func boltStroke(points []math2d.Vec2, k, scale float64) []render.Command {
	cmds := make([]render.Command, 0, (len(points)-1)*2)
	for s := 0; s+1 < len(points); s++ {
		depth := roadHalfAt(points[s].Y) / roadHitHalf
		cmds = append(cmds,
			render.DrawLine{
				BaseCommand: render.BaseCommand{LayerValue: lightningLayer},
				From:        points[s],
				To:          points[s+1],
				Width:       0.9 * depth * scale,
				Color:       render.RGBA(0.55, 0.7, 1, 0.28*k),
			},
			render.DrawLine{
				BaseCommand: render.BaseCommand{LayerValue: lightningLayer},
				From:        points[s],
				To:          points[s+1],
				Width:       0.28 * depth * scale,
				Color:       render.RGBA(0.95, 0.98, 1, 0.9*k),
			},
		)
	}
	return cmds
}

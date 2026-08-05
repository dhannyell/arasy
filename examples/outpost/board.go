package outpost

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

const (
	tileSize  = 8.0
	boardCols = 10
	boardRows = 8

	// The island floats: the front and right boundary drop this far before
	// the void, which is what sells the whole board as a solid object.
	cliffDepth = 7.0

	// Only the top fifth of the outer cliff keeps its lit tone; the bulk
	// below falls into shadow, which is what makes the wall read as tall.
	cliffLitDepth = cliffDepth * 0.2

	// Water sits below the grass, and the visible inner banks of that recess
	// are what make it a pool instead of a blue decal.
	waterDepth = 1.6

	// Where water meets the board's rim, a strip of ground-level shore stays
	// between the surface and the cliff. It is what closes the island's
	// silhouette: without it the corner of a water tile has no top surface to
	// come to a point with, and the outline reads as chopped off.
	shoreWidth = 1.6
)

// Layers, back to front. Flat strata are painted in order; everything that
// stands on the board -- units, trees, mountains, buildings -- shares
// layerActors and lets base-sort order it by where it meets the ground.
const (
	// The cliffs draw over the ground layer on purpose: the water surface
	// sits below ground level, so near the board's rim its far strip slides
	// under where the dirt stands -- the dirt in front has to win.
	layerGround = 1
	layerCliff  = 2
	layerCloud  = 3

	// Ground marks under the actors: the contact shadow and selection ring.
	// A mark lies on the ground, so anything standing anywhere correctly
	// draws over it -- no depth sorting needed.
	layerMarks = 4
	layerHover = 5

	layerActors = 6
)

type terrain byte

const (
	terrainGrass terrain = iota
	terrainForest
	terrainWater
	terrainMountain
	terrainPeak // a taller, two-tier mountain
	terrainBuilding
)

// boardMap is authored as text so the layout can be read at a glance:
// 'M' tall mountain, 'm' mountain, '~' water, 'f' forest, 'B' building,
// '.' grass.
var boardMap = [boardRows]string{
	"MMm....~~~",
	"Mm..f..~~~",
	"....f...~~",
	"..B.....~~",
	"....B.....",
	".f....B...",
	"...f......",
	".f....f...",
}

func terrainAt(i, j int) terrain {
	if i < 0 || j < 0 || i >= boardCols || j >= boardRows {
		return terrainGrass
	}
	switch boardMap[j][i] {
	case 'M':
		return terrainPeak
	case 'm':
		return terrainMountain
	case '~':
		return terrainWater
	case 'f':
		return terrainForest
	case 'B':
		return terrainBuilding
	default:
		return terrainGrass
	}
}

// boardRect is the whole board on the ground, the rectangle everything else
// is authored inside.
func boardRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: boardCols * tileSize, Y: boardRows * tileSize})
}

func tileRect(i, j int) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: float64(i) * tileSize, Y: float64(j) * tileSize},
		math2d.Vec2{X: tileSize, Y: tileSize},
	)
}

func tileCenter(i, j int) math2d.Vec2 {
	return math2d.Vec2{X: (float64(i) + 0.5) * tileSize, Y: (float64(j) + 0.5) * tileSize}
}

// tileAt is the inverse of tileCenter's layout: which tile a world point is
// in, and whether it is on the board at all. The caller hands it a point that
// already came back through ScreenToWorld, so the projection never enters
// tile math.
func tileAt(world math2d.Vec2) (int, int, bool) {
	i := int(world.X / tileSize)
	j := int(world.Y / tileSize)
	if world.X < 0 || world.Y < 0 || i >= boardCols || j >= boardRows {
		return 0, 0, false
	}
	return i, j, true
}

// The palette. Muted and flat on purpose: the scene's depth comes from the
// projection, and flat fills keep every face's angle legible.
var (
	grassA       = render.RGBA8(126, 156, 90, 255)
	grassB       = render.RGBA8(116, 146, 83, 255)
	forestGround = render.RGBA8(103, 133, 75, 255)
	tileStroke   = render.RGBA(0.18, 0.26, 0.12, 0.16)

	cliffSouth     = render.RGBA8(121, 94, 72, 255)
	cliffSouthDeep = render.RGBA8(95, 71, 54, 255)
	cliffEast      = render.RGBA8(98, 74, 58, 255)
	cliffEastDeep  = render.RGBA8(76, 57, 44, 255)

	waterFill = render.RGBA8(62, 118, 166, 255)
	bankLit   = render.RGBA8(94, 74, 56, 255) // banks the light reaches (south-facing)
	bankShade = render.RGBA8(78, 61, 47, 255) // banks in shadow (east-, north- and west-facing)
	shoreTop  = render.RGBA8(148, 116, 86, 255)

	rockSouth     = render.RGBA8(122, 113, 104, 255)
	rockEast      = render.RGBA8(103, 95, 88, 255)
	rockSnow      = render.RGBA8(235, 239, 242, 255)
	rockSnowShade = render.RGBA8(206, 213, 220, 255)

	buildingStyle = render.BlockStyle{
		Top:   render.RGBA8(211, 204, 191, 255),
		WallX: render.RGBA8(157, 148, 136, 255), // the east face, deeper in shade
		WallY: render.RGBA8(185, 176, 163, 255), // the south face, toward the light
	}
	windowDark = render.RGBA8(58, 70, 92, 255)
	windowLit  = render.RGBA8(255, 213, 122, 255)

	trunkColor  = render.RGBA8(104, 78, 54, 255)
	canopyColor = render.RGBA8(62, 116, 70, 255)
	canopyCrown = render.RGBA8(80, 138, 84, 255)

	standShadow = render.RGBA(0.02, 0.05, 0.03, 0.25)
)

func inset(rect math2d.Rect, amount float64) math2d.Rect {
	return math2d.NewRectFromCorners(
		rect.Min.Add(math2d.Vec2{X: amount, Y: amount}),
		rect.Max.Sub(math2d.Vec2{X: amount, Y: amount}),
	)
}

// waterSurfaceRect is the part of a water tile the surface actually covers:
// the whole tile, except that on the board's rim it pulls back behind a
// ground-level shore strip. The strip is what closes the island's outline --
// without it the corner of a water tile has no top surface to come to a point
// with, and the silhouette reads as chopped off.
func waterSurfaceRect(i, j int) math2d.Rect {
	surface := tileRect(i, j)
	if j == 0 {
		surface.Min.Y += shoreWidth
	}
	if i == 0 {
		surface.Min.X += shoreWidth
	}
	if j == boardRows-1 {
		surface.Max.Y -= shoreWidth
	}
	if i == boardCols-1 {
		surface.Max.X -= shoreWidth
	}
	return surface
}

// buildStaticBoard emits the terrain that never changes and never needs depth
// sorting: tile tops, the water recess and the island's outer cliffs. The
// standing props live in spawnProps instead, as entities the painter sort can
// order against the units.
func buildStaticBoard(p render.Projection) []render.Command {
	var commands []render.Command

	commands = waterCommands(commands, p)

	for j := range boardRows {
		for i := 0; i < boardCols; i++ {
			if terrainAt(i, j) == terrainWater {
				continue // painted by waterCommands
			}
			fill := grassA
			if (i+j)%2 == 1 {
				fill = grassB
			}
			if terrainAt(i, j) == terrainForest {
				fill = forestGround
			}
			commands = append(commands, render.DrawRect{
				Rect:        tileRect(i, j),
				Fill:        fill,
				Stroke:      tileStroke,
				StrokeWidth: 0.18,
				BaseCommand: render.BaseCommand{LayerValue: layerGround},
			})
		}
	}

	// The outer cliffs: one south face under the front edge, one east face
	// down the right edge. Two tones per face, because a lit band over a
	// darker one reads as depth where a single flat wall reads as a sticker.
	const width, height = boardCols * tileSize, boardRows * tileSize
	for _, side := range []struct {
		a, b        math2d.Vec2
		fill, lower render.Color
	}{
		{math2d.Vec2{X: 0, Y: height}, math2d.Vec2{X: width, Y: height}, cliffSouth, cliffSouthDeep},
		{math2d.Vec2{X: width, Y: 0}, math2d.Vec2{X: width, Y: height}, cliffEast, cliffEastDeep},
	} {
		commands = append(commands,
			p.Wall(side.a, side.b, -cliffLitDepth, 0, side.fill, layerCliff),
			p.Wall(side.a, side.b, -cliffDepth, -cliffLitDepth, side.lower, layerCliff),
		)
	}

	return commands
}

// waterCommands paints the whole recess in three passes over every water
// tile: shore tops, then bank walls, then the surface. Order between tiles
// stops mattering inside a pass, because no pass overlaps itself.
func waterCommands(commands []render.Command, p render.Projection) []render.Command {
	eachWaterTile := func(visit func(i, j int, rect, surface math2d.Rect)) {
		for j := 0; j < boardRows; j++ {
			for i := 0; i < boardCols; i++ {
				if terrainAt(i, j) == terrainWater {
					visit(i, j, tileRect(i, j), waterSurfaceRect(i, j))
				}
			}
		}
	}

	eachWaterTile(func(_, _ int, rect, surface math2d.Rect) {
		if surface != rect {
			commands = append(commands, render.DrawRect{
				Rect:        rect,
				Fill:        shoreTop,
				BaseCommand: render.BaseCommand{LayerValue: layerGround},
			})
		}
	})

	// The visible inner banks are the walls of the recess, drawn only where
	// the neighbour is land (terrainAt reports grass past the rim, so the
	// board's edge counts as land too). Far banks -- the walls the camera
	// looks into across the water -- go on the ground layer. Near banks are
	// the ledge under the land in front of the water; that land's tiles are
	// painted after the water, so the ledge rides the cliff layer to win.
	// Adjacent walls share their mitre edge by construction: each one starts
	// and ends on a corner of the surface rectangle.
	eachWaterTile(func(i, j int, _, surface math2d.Rect) {
		if terrainAt(i, j-1) != terrainWater {
			commands = append(commands, p.Wall(surface.Min, math2d.Vec2{X: surface.Max.X, Y: surface.Min.Y}, -waterDepth, 0, bankLit, layerGround))
		}
		if terrainAt(i-1, j) != terrainWater {
			commands = append(commands, p.Wall(surface.Min, math2d.Vec2{X: surface.Min.X, Y: surface.Max.Y}, -waterDepth, 0, bankShade, layerGround))
		}
		if terrainAt(i+1, j) != terrainWater {
			commands = append(commands, p.Wall(math2d.Vec2{X: surface.Max.X, Y: surface.Min.Y}, surface.Max, -waterDepth, 0, bankShade, layerCliff))
		}
		if terrainAt(i, j+1) != terrainWater {
			commands = append(commands, p.Wall(math2d.Vec2{X: surface.Min.X, Y: surface.Max.Y}, surface.Max, -waterDepth, 0, bankShade, layerCliff))
		}
	})

	eachWaterTile(func(_, _ int, _, surface math2d.Rect) {
		commands = append(commands, render.DrawRect{
			Rect:        p.LiftRect(surface, -waterDepth),
			Fill:        waterFill,
			BaseCommand: render.BaseCommand{LayerValue: layerGround},
		})
	})

	return commands
}

// spawnProps plants everything that stands on the board: trees as trunk
// entities with canopies attached above them, mountains and buildings as
// CommandSprite entities. All of them share layerActors with the units, and
// base-sort at the tile they stand on is what keeps a unit behind a building
// it has not passed yet and in front of one it has.
func spawnProps(world *scene.World, p render.Projection) {
	for j := 0; j < boardRows; j++ {
		for i := 0; i < boardCols; i++ {
			switch terrainAt(i, j) {
			case terrainForest:
				spawnTree(world, i, j)
			case terrainMountain:
				spawnCommandProp(world, i, j, mountainCommands(p, i, j, false))
			case terrainPeak:
				spawnCommandProp(world, i, j, mountainCommands(p, i, j, true))
			case terrainBuilding:
				spawnCommandProp(world, i, j, buildingCommands(p, i, j))
			}
		}
	}
}

// spawnCommandProp is a volumetric prop drawn with raw commands: the entity's
// position at the tile centre is where it meets the ground, which is all the
// painter sort needs to order it against everything walking past.
func spawnCommandProp(world *scene.World, i, j int, commands []render.Command) {
	world.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: tileCenter(i, j)},
		CommandSprite: &scene.CommandSprite{
			Layer:    layerActors,
			Commands: commands,
		},
	})
}

// spawnTree plants a trunk with two canopies attached above it. The canopies
// hang up the projection's lift axis -- "straight up on screen", which under
// this view is not the world's -Y (the isometric art trap docs/en/2-5d.md warns
// about) -- and the Attachment keeps the whole tree one thing in the painter
// order, sorting where the trunk meets the ground.
func spawnTree(world *scene.World, i, j int) {
	const (
		trunkHalfWidth = 0.55
		trunkHeight    = 2.6
		canopyHeight   = 3.7 // how far up the lift axis the big canopy hangs
		canopyRadius   = 2.3
		crownHeight    = 5.6 // the smaller ball that gives the tree a tip
		crownRadius    = 1.4
	)

	foot := tileCenter(i, j)
	trunk := world.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: foot},
		RectSprite: &scene.RectSprite{
			Rect: math2d.NewRectFromCorners(
				foot.Sub(math2d.Vec2{X: trunkHalfWidth, Y: trunkHeight}),
				foot.Add(math2d.Vec2{X: trunkHalfWidth, Y: 0.2}),
			),
			BaseSprite: scene.BaseSprite{Layer: layerActors, Color: trunkColor},
		},
	})

	for _, ball := range []struct {
		height, radius float64
		fill           render.Color
	}{
		{canopyHeight, canopyRadius, canopyColor},
		{crownHeight, crownRadius, canopyCrown},
	} {
		world.SpawnAttached(trunk, math2d.Vec2{}, ball.height, scene.Bundle{
			CircleSprite: &scene.CircleSprite{
				Radius:     ball.radius,
				BaseSprite: scene.BaseSprite{Layer: layerActors, Color: ball.fill},
			},
		})
	}
}

// mountainCommands raises a faceted pyramid: two triangular faces from the
// tile's near edges to a single lifted apex, each its own flat tone, with a
// snow triangle riding the top of each face. Same extrusion idea as a block,
// with the top rectangle collapsed to a point.
func mountainCommands(p render.Projection, i, j int, tall bool) []render.Command {
	rect := inset(tileRect(i, j), 0.4)

	height := 3.4
	if tall {
		height = 5.4
	}
	apex := p.Lift(rect.Center(), height)

	facet := func(a, b math2d.Vec2, fill render.Color) render.DrawPolygon {
		return render.DrawPolygon{
			Points:      []math2d.Vec2{a, b, apex},
			Fill:        fill,
			BaseCommand: render.BaseCommand{LayerValue: layerActors},
		}
	}
	toward := func(pt math2d.Vec2, t float64) math2d.Vec2 {
		return pt.Add(apex.Sub(pt).Mul(t))
	}
	snow := func(a, b math2d.Vec2, fill render.Color) render.DrawPolygon {
		return facet(toward(a, 0.55), toward(b, 0.55), fill)
	}

	southA := math2d.Vec2{X: rect.Min.X, Y: rect.Max.Y}
	eastA := math2d.Vec2{X: rect.Max.X, Y: rect.Min.Y}
	corner := rect.Max

	return []render.Command{
		facet(eastA, corner, rockEast),
		facet(southA, corner, rockSouth),
		snow(eastA, corner, rockSnowShade),
		snow(southA, corner, rockSnow),
	}
}

func buildingCommands(p render.Projection, i, j int) []render.Command {
	rect := inset(tileRect(i, j), 1.3)
	const height = 4.4

	commands := p.ExtrudeRect(nil, rect, 0, height, buildingStyle, layerActors)

	// Windows live on the south face: the same sweep as the face itself, so
	// they stay glued to the wall whatever the projection does.
	south := func(u float64) math2d.Vec2 {
		return math2d.Vec2{X: rect.Min.X + u*rect.Width(), Y: rect.Max.Y}
	}
	window := 0
	for _, zs := range [][2]float64{{2.7, 3.5}, {1.1, 1.9}} {
		for _, us := range [][2]float64{{0.14, 0.3}, {0.42, 0.58}, {0.7, 0.86}} {
			fill := windowDark
			// A few windows stay lit, deterministically per window, for life.
			if (i*7+j*13+window*5)%3 == 0 {
				fill = windowLit
			}
			window++
			commands = append(commands, p.Wall(south(us[0]), south(us[1]), zs[0], zs[1], fill, layerActors))
		}
	}

	// A rooftop box breaks the silhouette, which is most of what makes a
	// building read as architecture instead of a crate.
	return p.ExtrudeRect(commands, inset(rect, 1.8), height, height+1.1, buildingStyle, layerActors)
}

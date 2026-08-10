package main

import (
	"cmp"
	"embed"
	"io/fs"
	"math"
	"slices"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/dmath"
	"github.com/dhannyell/arasy/internal/math3d"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

//go:embed assets
var assetsFS embed.FS

func loadManifest() (asset.Manifest, error) {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return asset.Manifest{}, err
	}
	return asset.Load(sub)
}

const (
	cubeSize   = 10
	halfExtent = cubeSize / 2.0

	maxCubes = 512

	gridSize    = 3
	gridSpacing = 25.0
)

// topFacePair indexes the quad face pair drawn textured instead of colored.
const topFacePair = 1

// topQuad lists the top-face vertices in SOURCE corner order: the texture
// top-left lands on vertex 7, then top-right, bottom-right, bottom-left.
var topQuad = [4]int{7, 6, 5, 4}

var faceColors = [6]render.Color{
	render.RGBA(0.9, 0.3, 0.3, 1), // bottom
	render.RGBA(0.3, 0.9, 0.3, 1), // top
	render.RGBA(0.3, 0.3, 0.9, 1), // rear
	render.RGBA(0.9, 0.9, 0.3, 1), // right
	render.RGBA(0.9, 0.3, 0.9, 1), // front
	render.RGBA(0.3, 0.9, 0.9, 1), // left
}

type poc3d struct {
	runtime.StateSlot[SceneState]
	cube     Cube
	orbit    orbitCamera
	viewport math2d.Rect
}

type Cube struct {
	Vertices [8]math3d.Vec3
	Faces    [12][3]int
}

type CubeState struct {
	Position      math3d.Vec3
	Angle         float64
	PreviousAngle float64
}

type SceneState struct {
	Cubes     [maxCubes]CubeState
	CubeCount int
}

func (p *poc3d) Setup() runtime.Config {
	assets, err := loadManifest()
	if err != nil {
		panic(err)
	}
	p.viewport = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080})
	// Same pose the fixed camera had: eye at {70,-70,45} looking at the origin.
	p.orbit = newOrbitCamera(math3d.Vec3{}, -math.Pi/4, 0.43, 110)

	return runtime.Config{
		Camera: render.Camera{
			World:    p.viewport,
			Viewport: p.viewport,
		},
		Clear:                render.RGBA(0, 0, 0, 1),
		SimulationHz:         60,
		FPS:                  60,
		RenderInterpolation:  true,
		Assets:               assets,
		HistoryLen:           600,
		DisableDefaultSplash: true,
	}
}

func newCubeMesh() Cube {
	return Cube{
		Vertices: [8]math3d.Vec3{
			{X: -halfExtent, Y: -halfExtent, Z: -halfExtent},
			{X: halfExtent, Y: -halfExtent, Z: -halfExtent},
			{X: halfExtent, Y: halfExtent, Z: -halfExtent},
			{X: -halfExtent, Y: halfExtent, Z: -halfExtent},
			{X: -halfExtent, Y: -halfExtent, Z: halfExtent},
			{X: halfExtent, Y: -halfExtent, Z: halfExtent},
			{X: halfExtent, Y: halfExtent, Z: halfExtent},
			{X: -halfExtent, Y: halfExtent, Z: halfExtent},
		},
		Faces: [12][3]int{
			{0, 2, 1}, {0, 3, 2}, // bottom (Z = -halfExtent)
			{4, 5, 6}, {4, 6, 7}, // top (Z = +halfExtent)
			{0, 1, 5}, {0, 5, 4}, // rear (Y = -halfExtent)
			{1, 2, 6}, {1, 6, 5}, // right (X = +halfExtent)
			{2, 3, 7}, {2, 7, 6}, // front (Y = +halfExtent)
			{3, 0, 4}, {3, 4, 7}, // left (X = -halfExtent)
		},
	}
}

func (p *poc3d) Init(ctx *runtime.UpdateContext) {
	p.cube = newCubeMesh()

	p.State = SceneState{}
	for gy := range gridSize {
		for gx := range gridSize {
			i := gy*gridSize + gx
			// Stagger the start angles so the spins stay visually distinct.
			angle := float64(i) * 0.3
			p.State.Cubes[i] = CubeState{
				Position: math3d.Vec3{
					X: (float64(gx) - (gridSize-1)/2.0) * gridSpacing,
					Y: (float64(gy) - (gridSize-1)/2.0) * gridSpacing,
				},
				Angle:         angle,
				PreviousAngle: angle,
			}
		}
	}
	p.State.CubeCount = gridSize * gridSize
}

func (p *poc3d) Update(ctx *runtime.UpdateContext, dt float64) {
	const spinSpeed = 0.03 // radians per fixed tick

	p.orbit.update(ctx.Pointer)

	for i := 0; i < p.State.CubeCount; i++ {
		c := &p.State.Cubes[i]
		c.PreviousAngle = c.Angle
		c.Angle += spinSpeed
		if c.Angle >= 2*math.Pi {
			c.Angle -= 2 * math.Pi
			// Shift the previous angle too so render interpolation stays continuous.
			c.PreviousAngle -= 2 * math.Pi
		}
	}
}

func (p *poc3d) Render(ctx *runtime.RenderContext) []render.Command {
	cubes := p.State.Cubes[:p.State.CubeCount]
	return paintScene(p.orbit.camera(p.viewport), p.cube, cubes, ctx.Alpha, ctx.Assets.Texture("checker"))
}

type painted struct {
	depth float64
	cmd   render.Command
}

func paintScene(cam math3d.Camera, mesh Cube, cubes []CubeState, alpha float64, tex render.TextureID) []render.Command {
	paint := make([]painted, 0, len(cubes)*len(mesh.Faces))
	for i := range cubes {
		paint = appendCubeFaces(paint, cam, mesh, cubes[i], alpha, tex)
	}

	slices.SortStableFunc(paint, func(a, b painted) int {
		return cmp.Compare(b.depth, a.depth)
	})
	commands := make([]render.Command, 0, len(paint))
	for _, pf := range paint {
		commands = append(commands, pf.cmd)
	}
	return commands
}

// appendCubeFaces projects one cube and appends its visible faces.
// A face with any vertex behind Near is dropped whole (no clipping in the PoC).
func appendCubeFaces(paint []painted, cam math3d.Camera, mesh Cube, st CubeState, alpha float64, tex render.TextureID) []painted {
	visualAngle := st.PreviousAngle + (st.Angle-st.PreviousAngle)*alpha
	cosA := dmath.Cos(visualAngle)
	sinA := dmath.Sin(visualAngle)

	// Rotate around Z, translate, and project every cube vertex.
	var screen [8]math2d.Vec2
	var depth [8]float64
	var valid [8]bool
	for i, v := range mesh.Vertices {
		world := math3d.Vec3{
			X: v.X*cosA - v.Y*sinA + st.Position.X,
			Y: v.X*sinA + v.Y*cosA + st.Position.Y,
			Z: v.Z + st.Position.Z,
		}
		screen[i], depth[i], valid[i] = cam.ProjectPoint(world)
	}

	for i, f := range mesh.Faces {
		if i/2 == topFacePair {
			continue // the top face draws as a textured quad below
		}
		if !valid[f[0]] || !valid[f[1]] || !valid[f[2]] {
			continue
		}
		if signedArea(screen[f[0]], screen[f[1]], screen[f[2]]) >= 0 {
			continue
		}
		centroid := (depth[f[0]] + depth[f[1]] + depth[f[2]]) / 3
		paint = append(paint, painted{
			depth: centroid,
			cmd: render.DrawTriangles{
				Vertices: []math2d.Vec2{screen[f[0]], screen[f[1]], screen[f[2]]},
				Indices:  []uint32{0, 1, 2},
				Color:    faceColors[i/2],
			},
		})
	}

	// The quad corner order (source TL,TR,BR,BL) runs clockwise seen from
	// outside, the mirror of the CCW triangle convention, so the visibility
	// sign flips: positive area means the face is in front.
	q0, q1, q2, q3 := topQuad[0], topQuad[1], topQuad[2], topQuad[3]
	if valid[q0] && valid[q1] && valid[q2] && valid[q3] &&
		signedArea(screen[q0], screen[q1], screen[q2]) > 0 {
		centroid := (depth[q0] + depth[q1] + depth[q2] + depth[q3]) / 4
		paint = append(paint, painted{
			depth: centroid,
			cmd: render.DrawImageQuad{
				Texture: tex,
				Corners: [4]math2d.Vec2{screen[q0], screen[q1], screen[q2], screen[q3]},
			},
		})
	}

	return paint
}

func signedArea(a, b, c math2d.Vec2) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

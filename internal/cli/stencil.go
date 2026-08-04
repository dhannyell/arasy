package cli

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
	"github.com/spf13/cobra"
)

// The stencil is the contract between code and art: a guide sheet rendered by
// the same camera, projection and extrusion the game will use, so an artist
// can drop it under their canvas and read the angles and pixel sizes off it
// instead of asking the programmer. The flags mirror the fields of a game's
// Setup one for one.

var (
	stencilOut    string
	stencilView   string
	stencilDepth  float64
	stencilTilePx float64
	stencilLight  string
	stencilTilt   float64
	stencilSoft   float64
)

var stencilCmd = &cobra.Command{
	Use:   "stencil",
	Short: "render an art guide sheet for a camera projection",
	Long: "Render a PNG guide sheet -- tile grid, unit cube, lift and light --\n" +
		"under the exact projection a game declares, so sprite art can be drawn\n" +
		"at the right angle and scale. Copy the values from the game's Setup:\n" +
		"--view takes the classic \"2:1\" ratio or the raw affine \"a,c,b,d\"\n" +
		"(screenX = a*x + c*y, screenY = b*x + d*y), --depth-scale takes a\n" +
		"tilted top-down camera instead.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		camera, err := stencilCamera()
		if err != nil {
			return err
		}
		lightDir, err := parseStencilVec(stencilLight)
		if err != nil {
			return fmt.Errorf("--light-dir: %w", err)
		}
		light := render.ProjectionLight{Direction: lightDir, Tilt: stencilTilt, Softness: stencilSoft}

		game := &stencilGame{camera: camera, light: light}
		width, height := stencilSheetSize(camera.View)
		if err := raster.Run(game, raster.Options{
			Width: width, Height: height, Frames: 1, FrameDT: 1.0 / 60.0,
			PNG: stencilOut, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}); err != nil {
			return err
		}

		// The same numbers, in artist units, for whoever never opens the file.
		tile := game.tilePixels()
		lift := game.liftPixels()
		_, err = fmt.Fprintf(cmd.OutOrStdout(),
			"wrote %s\n  ground tile: %.0f x %.0f px\n  one unit of height: %.0f px along (%.2f, %.2f)\n",
			stencilOut, tile.X, tile.Y, lift.Len(), norm(lift).X, norm(lift).Y)
		return err
	},
}

func init() {
	stencilCmd.Flags().StringVarP(&stencilOut, "out", "o", "stencil.png", "output PNG path")
	stencilCmd.Flags().StringVar(&stencilView, "view", "2:1", `Camera.View: "W:H" iso ratio, or "a,c,b,d" raw affine; "" for none`)
	stencilCmd.Flags().Float64Var(&stencilDepth, "depth-scale", 0, "Camera.DepthScale for a tilted top-down camera (used when --view is empty)")
	stencilCmd.Flags().Float64Var(&stencilTilePx, "tile", 64, "width in pixels of one ground unit's projected tile")
	stencilCmd.Flags().StringVar(&stencilLight, "light-dir", "-1,-1", "ProjectionLight.Direction, x,y")
	stencilCmd.Flags().Float64Var(&stencilTilt, "light-tilt", 0.3, "ProjectionLight.Tilt, 0 overhead to 1 grazing")
	stencilCmd.Flags().Float64Var(&stencilSoft, "light-softness", 0.5, "ProjectionLight.Softness, 0 point to 1 diffuse")
	rootCmd.AddCommand(stencilCmd)
}

func stencilCamera() (render.Camera, error) {
	camera := render.Camera{Aspect: render.AspectFit}
	switch {
	case stencilView != "":
		view, err := parseStencilView(stencilView)
		if err != nil {
			return camera, fmt.Errorf("--view: %w", err)
		}
		camera.View = view
	case stencilDepth != 0:
		camera.DepthScale = stencilDepth
	}
	return camera, nil
}

func parseStencilView(value string) (math2d.Affine, error) {
	if ratio := strings.Split(value, ":"); len(ratio) == 2 {
		w, errW := strconv.ParseFloat(ratio[0], 64)
		h, errH := strconv.ParseFloat(ratio[1], 64)
		if errW != nil || errH != nil || w <= 0 || h <= 0 {
			return math2d.Affine{}, fmt.Errorf("invalid iso ratio %q", value)
		}
		// The classic diamond: screenX = x - y, screenY = (x + y) * h/w.
		return math2d.Affine{A: 1, C: -1, B: h / w, D: h / w}, nil
	}

	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return math2d.Affine{}, fmt.Errorf("want \"W:H\" or four numbers a,c,b,d, got %q", value)
	}
	numbers := make([]float64, 4)
	for i, part := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return math2d.Affine{}, fmt.Errorf("invalid number %q", part)
		}
		numbers[i] = n
	}
	return math2d.Affine{A: numbers[0], C: numbers[1], B: numbers[2], D: numbers[3]}, nil
}

func parseStencilVec(value string) (math2d.Vec2, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return math2d.Vec2{}, fmt.Errorf("want x,y, got %q", value)
	}
	x, errX := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	y, errY := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errX != nil || errY != nil {
		return math2d.Vec2{}, fmt.Errorf("invalid numbers in %q", value)
	}
	return math2d.Vec2{X: x, Y: y}, nil
}

func norm(v math2d.Vec2) math2d.Vec2 {
	if length := v.Len(); length > 0 {
		return v.Mul(1 / length)
	}
	return v
}

package ffcli

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/dhannyell/frameforge/asset"
	"github.com/spf13/cobra"
)

var assetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "asset tooling",
}

var assetsValidateCmd = &cobra.Command{
	Use:   "validate <dir> [dir...]",
	Short: "validate one or more layered asset directories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manifest, err := validateAssetDirs(args)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "manifest ok: %d textures, %d fonts, %d audio clips\n", len(manifest.Textures()), len(manifest.Fonts()), len(manifest.AudioClips()))
		return nil
	},
}

func validateAssetDirs(dirs []string) (asset.Manifest, error) {
	layers := make([]fs.FS, 0, len(dirs))
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return asset.Manifest{}, fmt.Errorf("asset directory %q does not exist", dir)
		}
		layers = append(layers, os.DirFS(dir))
	}
	return asset.Load(layers...)
}

func init() {
	assetsCmd.AddCommand(assetsValidateCmd)
	rootCmd.AddCommand(assetsCmd)
}

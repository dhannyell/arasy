package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhannyell/arasy/internal/svggen"
	"github.com/spf13/cobra"
)

type svgFlags struct {
	output      string
	packageName string
	name        string
	fonts       []string
	strict      bool
}

var svgOptions svgFlags

var svgCmd = &cobra.Command{
	Use:   "svg <source.svg>",
	Short: "compile an animated SVG into native render commands",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return compileSVG(args[0], svgOptions)
	},
}

func compileSVG(source string, options svgFlags) error {
	if options.output == "" {
		return fmt.Errorf("svg: --output is required")
	}
	fonts, err := parseFontMappings(options.fonts)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("svg: read %q: %w", source, err)
	}
	document, err := svggen.Parse(data, svggen.Options{Strict: options.strict, Fonts: fonts})
	if err != nil {
		return err
	}
	generated, err := svggen.Generate(document, svggen.GenerateOptions{
		Package: options.packageName,
		Name:    options.name,
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(options.output), 0o755); err != nil {
		return fmt.Errorf("svg: create output directory: %w", err)
	}
	if err := os.WriteFile(options.output, generated, 0o644); err != nil {
		return fmt.Errorf("svg: write %q: %w", options.output, err)
	}
	return nil
}

func parseFontMappings(values []string) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("svg: --font expects FAMILY=MANIFEST_NAME, got %q", value)
		}
		family, manifest := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, exists := result[family]; exists {
			return nil, fmt.Errorf("svg: duplicate font mapping %q", family)
		}
		result[family] = manifest
	}
	return result, nil
}

func init() {
	svgCmd.Flags().StringVarP(&svgOptions.output, "output", "o", "", "generated Go output path")
	svgCmd.Flags().StringVar(&svgOptions.packageName, "package", "", "generated Go package")
	svgCmd.Flags().StringVar(&svgOptions.name, "name", "", "exported generated animation name")
	svgCmd.Flags().StringArrayVar(&svgOptions.fonts, "font", nil, "font mapping FAMILY=MANIFEST_NAME (repeatable)")
	svgCmd.Flags().BoolVar(&svgOptions.strict, "strict", true, "reject unsupported SVG and CSS")
	_ = svgCmd.MarkFlagRequired("output")
	_ = svgCmd.MarkFlagRequired("package")
	_ = svgCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(svgCmd)
}

package svggen

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dhannyell/arasy/math2d"
)

var acceptedAttributes = map[string]bool{
	"id": true, "class": true, "style": true, "viewBox": true, "preserveAspectRatio": true,
	"x": true, "y": true, "x1": true, "y1": true, "x2": true, "y2": true,
	"cx": true, "cy": true, "r": true, "rx": true, "ry": true,
	"width": true, "height": true, "points": true, "d": true,
	"transform": true, "opacity": true, "fill": true, "stroke": true,
	"stroke-width": true, "stroke-linecap": true, "stroke-linejoin": true,
	"stroke-miterlimit": true, "font-family": true, "font-size": true,
	"font-weight": true, "text-anchor": true, "letter-spacing": true,
	"pathLength": true, "transform-box": true, "transform-origin": true,
	"animation": true, "animation-name": true, "animation-duration": true,
	"animation-delay": true, "animation-iteration-count": true,
	"animation-direction": true, "animation-fill-mode": true,
	"animation-timing-function": true, "stroke-dasharray": true,
	"stroke-dashoffset": true, "offset-path": true, "offset-distance": true,
	"filter": true, "type": true, "media": true,
}

func validateDocument(document *Document, options Options) error {
	viewBox, err := parseNumberList(document.Root.Attributes["viewBox"])
	if err != nil || len(viewBox) != 4 || viewBox[2] <= 0 || viewBox[3] <= 0 {
		return fmt.Errorf("svg:%d: svg.viewBox: expected four finite numbers with positive size", document.Root.Line)
	}
	document.ViewBox = math2d.NewRect(math2d.Vec2{X: viewBox[0], Y: viewBox[1]}, math2d.Vec2{X: viewBox[2], Y: viewBox[3]})
	if value := document.Root.Attributes["preserveAspectRatio"]; value != "" && value != "xMidYMid meet" {
		return fmt.Errorf("svg:%d: preserveAspectRatio %q is outside the supported subset", document.Root.Line, value)
	}
	css := ""
	var walk func(*Element) error
	walk = func(element *Element) error {
		for key := range element.Attributes {
			if !acceptedAttributes[key] {
				if options.Strict {
					return fmt.Errorf("svg:%d: %s.%s is outside the supported subset", element.Line, element.Name, key)
				}
				delete(element.Attributes, key)
			}
		}
		if element.Name == "style" {
			if element.Attributes["media"] != "" ||
				element.Attributes["type"] != "" && element.Attributes["type"] != "text/css" {
				return fmt.Errorf("svg:%d: unsupported style media or type", element.Line)
			}
			css += element.Text + "\n"
		}
		if data := element.Attributes["d"]; data != "" {
			if _, err := ParsePathData(data); err != nil {
				return fmt.Errorf("svg:%d: %s.d: %w", element.Line, element.Name, err)
			}
		}
		if transform := element.Attributes["transform"]; transform != "" {
			if _, err := ParseCSSTransform(transform); err != nil {
				return fmt.Errorf("svg:%d: %s.transform: %w", element.Line, element.Name, err)
			}
		}
		for _, child := range element.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(document.Root); err != nil {
		return err
	}
	stylesheet, err := parseCSS(css, options.Strict)
	if err != nil {
		return err
	}
	document.Styles = stylesheet
	applyStyles(document.Root, stylesheet)
	return validateFonts(document)
}

func validateFonts(document *Document) error {
	var walk func(*Element) error
	walk = func(element *Element) error {
		if element.Name == "text" || element.Name == "tspan" {
			family := strings.Trim(element.Attributes["font-family"], ` "'`)
			if family != "" {
				if _, ok := document.Fonts[family]; !ok {
					return fmt.Errorf("svg:%d: font-family %q requires --font mapping", element.Line, family)
				}
			}
		}
		for _, child := range element.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(document.Root)
}

func parseNumberList(value string) ([]float64, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	result := make([]float64, 0, len(fields))
	for _, field := range fields {
		number, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || math.Abs(number) > MaxMagnitude {
			return nil, fmt.Errorf("invalid finite number %q", field)
		}
		result = append(result, number)
	}
	return result, nil
}

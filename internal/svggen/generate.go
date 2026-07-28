package svggen

import (
	"bytes"
	"fmt"
	"go/format"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// GenerateOptions controls the public surface emitted for one parsed SVG.
type GenerateOptions struct {
	Package string
	Name    string
}

// Generate emits deterministic, gofmt-formatted Go source. The generated
// program contains only native animation and render data; SVG, CSS, and
// embedded font bytes are never carried into the executable.
func Generate(document *Document, options GenerateOptions) ([]byte, error) {
	if document == nil || document.Root == nil {
		return nil, fmt.Errorf("svg: nil document")
	}
	if !validIdentifier(options.Package) || !validExportedIdentifier(options.Name) {
		return nil, fmt.Errorf("svg: package and exported name must be valid Go identifiers")
	}
	g := generator{document: document, options: options}
	if err := g.collect(); err != nil {
		return nil, err
	}
	source, err := g.emit()
	if err != nil {
		return nil, err
	}
	if len(source) > MaxGeneratedData {
		return nil, fmt.Errorf("svg: generated source exceeds %d bytes", MaxGeneratedData)
	}
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("svg: format generated source: %w", err)
	}
	return formatted, nil
}

type generator struct {
	document *Document
	options  GenerateOptions
	nodes    []genNode
	paths    [][]PathCommand
	fonts    []genFont
}

type genFont struct {
	family, manifest, field string
}

type genNode struct {
	kind       string
	children   []int
	path       int
	motionPath int
	x, y       float64
	x2, y2     float64
	rx, ry     float64
	width      float64
	height     float64
	text       string
	fill       render.Color
	stroke     render.Color
	strokeW    float64
	fontField  string
	fontSize   float64
	spacing    float64
	align      string
	transform  math2d.Affine
	origin     math2d.Vec2
	opacity    float64
	pathLength float64
	lineCap    string
	lineJoin   string
	animation  genAnimation
}

type genAnimation struct {
	delay, duration float64
	curve           string
	opacity         []scalarStop
	dashOffset      []scalarStop
	distance        []scalarStop
	transform       []affineStop
}

type scalarStop struct {
	progress, value float64
}

type affineStop struct {
	progress float64
	value    math2d.Affine
}

func (g *generator) collect() error {
	families := make([]string, 0, len(g.document.Fonts))
	for family := range g.document.Fonts {
		families = append(families, family)
	}
	sort.Strings(families)
	for _, family := range families {
		g.fonts = append(g.fonts, genFont{
			family: family, manifest: g.document.Fonts[family], field: exportedIdentifier(family),
		})
	}
	_, err := g.collectElement(g.document.Root, inheritedStyle{
		fill: render.Color{}, stroke: render.Color{}, opacity: 1,
	})
	return err
}

type inheritedStyle struct {
	fill, stroke render.Color
	fontFamily   string
	fontSize     float64
	spacing      float64
	align        string
	opacity      float64
}

func (g *generator) collectElement(element *Element, inherited inheritedStyle) (int, error) {
	if element.Name == "style" {
		return -1, nil
	}
	node := genNode{
		kind: element.Name, path: -1, motionPath: -1, fill: inherited.fill,
		stroke: inherited.stroke, fontSize: inherited.fontSize, spacing: inherited.spacing,
		align: inherited.align, opacity: 1, transform: math2d.Identity(),
	}
	if value := element.Attributes["fill"]; value != "" {
		color, err := parseColor(value)
		if err != nil {
			return -1, lineError(element, "fill", err)
		}
		node.fill = color
	}
	if value := element.Attributes["stroke"]; value != "" {
		color, err := parseColor(value)
		if err != nil {
			return -1, lineError(element, "stroke", err)
		}
		node.stroke = color
	}
	node.opacity = numberDefault(element.Attributes["opacity"], 1)
	node.strokeW = numberDefault(element.Attributes["stroke-width"], 1)
	node.fontSize = numberDefault(element.Attributes["font-size"], node.fontSize)
	node.spacing = numberDefault(element.Attributes["letter-spacing"], node.spacing)
	node.align = element.Attributes["text-anchor"]
	if node.align == "" {
		node.align = inherited.align
	}
	family := strings.Trim(element.Attributes["font-family"], ` "'`)
	if family == "" {
		family = inherited.fontFamily
	}
	for _, font := range g.fonts {
		if font.family == family {
			node.fontField = font.field
		}
	}
	var err error
	node.x, err = numberAttribute(element, "x", 0)
	if err != nil {
		return -1, err
	}
	node.y, _ = numberAttribute(element, "y", 0)
	switch element.Name {
	case "rect":
		node.width, _ = numberAttribute(element, "width", 0)
		node.height, _ = numberAttribute(element, "height", 0)
		node.rx, _ = numberAttribute(element, "rx", 0)
	case "circle":
		node.x, _ = numberAttribute(element, "cx", 0)
		node.y, _ = numberAttribute(element, "cy", 0)
		node.rx, _ = numberAttribute(element, "r", 0)
		node.ry = node.rx
	case "ellipse":
		node.x, _ = numberAttribute(element, "cx", 0)
		node.y, _ = numberAttribute(element, "cy", 0)
		node.rx, _ = numberAttribute(element, "rx", 0)
		node.ry, _ = numberAttribute(element, "ry", 0)
	case "line":
		node.x, _ = numberAttribute(element, "x1", 0)
		node.y, _ = numberAttribute(element, "y1", 0)
		node.x2, _ = numberAttribute(element, "x2", 0)
		node.y2, _ = numberAttribute(element, "y2", 0)
	case "polyline", "polygon":
		commands, parseErr := pointsPath(element.Attributes["points"], element.Name == "polygon")
		if parseErr != nil {
			return -1, lineError(element, "points", parseErr)
		}
		node.path = g.addPath(commands)
	case "path":
		commands, parseErr := ParsePathData(element.Attributes["d"])
		if parseErr != nil {
			return -1, lineError(element, "d", parseErr)
		}
		node.path = g.addPath(commands)
		node.pathLength = numberDefault(element.Attributes["pathLength"], 0)
	case "text", "tspan":
		node.text = strings.TrimSpace(element.Text)
	}
	if value := element.Attributes["transform"]; value != "" {
		node.transform, err = ParseCSSTransform(value)
		if err != nil {
			return -1, lineError(element, "transform", err)
		}
	}
	if element.Attributes["transform-box"] == "fill-box" && element.Attributes["transform-origin"] == "center" {
		node.origin = nodeBounds(node).Center()
	}
	node.lineCap = lineCapName(element.Attributes["stroke-linecap"])
	node.lineJoin = lineJoinName(element.Attributes["stroke-linejoin"])
	node.animation, err = g.animationFor(element)
	if err != nil {
		return -1, err
	}
	if motion := element.Attributes["offset-path"]; motion != "" {
		pathData, ok := pathFunction(motion)
		if !ok {
			return -1, lineError(element, "offset-path", fmt.Errorf("expected path('...')"))
		}
		commands, parseErr := ParsePathData(pathData)
		if parseErr != nil {
			return -1, lineError(element, "offset-path", parseErr)
		}
		node.motionPath = g.addPath(commands)
	}

	index := len(g.nodes)
	g.nodes = append(g.nodes, node)
	childStyle := inheritedStyle{
		fill: node.fill, stroke: node.stroke, fontFamily: family, fontSize: node.fontSize,
		spacing: node.spacing, align: node.align, opacity: inherited.opacity * node.opacity,
	}
	for _, child := range element.Children {
		childIndex, childErr := g.collectElement(child, childStyle)
		if childErr != nil {
			return -1, childErr
		}
		if childIndex >= 0 {
			g.nodes[index].children = append(g.nodes[index].children, childIndex)
		}
	}
	return index, nil
}

func (g *generator) animationFor(element *Element) (genAnimation, error) {
	value := element.Attributes["animation"]
	if value == "" && element.Attributes["animation-name"] == "" {
		return genAnimation{}, nil
	}
	animation, err := parseAnimation(value, element.Attributes)
	if err != nil {
		return genAnimation{}, lineError(element, "animation", err)
	}
	frames, ok := g.document.Styles.Keyframes[animation.name]
	if !ok {
		return genAnimation{}, lineError(element, "animation", fmt.Errorf("unknown keyframes %q", animation.name))
	}
	result := genAnimation{delay: animation.delay, duration: animation.duration, curve: animation.curve}
	for _, frame := range frames {
		if value, ok := frame.Declarations["opacity"]; ok {
			result.opacity = append(result.opacity, scalarStop{frame.Progress, numberDefault(value, 1)})
		}
		if value, ok := frame.Declarations["stroke-dashoffset"]; ok {
			result.dashOffset = append(result.dashOffset, scalarStop{frame.Progress, numberDefault(value, 0)})
		}
		if value, ok := frame.Declarations["offset-distance"]; ok {
			result.distance = append(result.distance, scalarStop{frame.Progress, percentNumber(value)})
		}
		if value, ok := frame.Declarations["transform"]; ok {
			transform, parseErr := ParseCSSTransform(value)
			if parseErr != nil {
				return genAnimation{}, lineError(element, "animation transform", parseErr)
			}
			result.transform = append(result.transform, affineStop{frame.Progress, transform})
		}
	}
	return result, nil
}

type parsedAnimation struct {
	name, curve string
	duration    float64
	delay       float64
}

func parseAnimation(shorthand string, attributes map[string]string) (parsedAnimation, error) {
	result := parsedAnimation{name: attributes["animation-name"], curve: "ease.Linear"}
	if shorthand != "" {
		for _, field := range strings.Fields(shorthand) {
			switch {
			case looksLikeDuration(field):
				seconds, err := cssSeconds(field)
				if err != nil {
					return result, err
				}
				if result.duration == 0 {
					result.duration = seconds
				} else {
					result.delay = seconds
				}
			case field == "linear" || field == "ease" || field == "ease-in" ||
				field == "ease-out" || field == "ease-in-out" ||
				strings.HasPrefix(field, "cubic-bezier("):
				curve, err := curveExpression(field)
				if err != nil {
					return result, err
				}
				result.curve = curve
			case field == "both" || field == "forwards" || field == "normal" || field == "1":
			default:
				if result.name == "" {
					result.name = field
				}
			}
		}
	}
	if value := attributes["animation-duration"]; value != "" {
		result.duration, _ = cssSeconds(value)
	}
	if value := attributes["animation-delay"]; value != "" {
		result.delay, _ = cssSeconds(value)
	}
	if value := attributes["animation-timing-function"]; value != "" {
		result.curve, _ = curveExpression(value)
	}
	if result.name == "" || result.duration <= 0 {
		return result, fmt.Errorf("animation requires a name and positive duration")
	}
	return result, nil
}

func looksLikeDuration(value string) bool {
	if !(strings.HasSuffix(value, "ms") || strings.HasSuffix(value, "s")) || value == "" {
		return false
	}
	first := value[0]
	return first == '+' || first == '-' || first == '.' || first >= '0' && first <= '9'
}

func curveExpression(value string) (string, error) {
	switch value {
	case "linear":
		return "ease.Linear", nil
	case "ease":
		return "ease.MustCubicBezier(0.25, 0.1, 0.25, 1)", nil
	case "ease-in":
		return "ease.MustCubicBezier(0.42, 0, 1, 1)", nil
	case "ease-out":
		return "ease.MustCubicBezier(0, 0, 0.58, 1)", nil
	case "ease-in-out":
		return "ease.MustCubicBezier(0.42, 0, 0.58, 1)", nil
	}
	if strings.HasPrefix(value, "cubic-bezier(") && strings.HasSuffix(value, ")") {
		values, err := parseNumberList(value[len("cubic-bezier(") : len(value)-1])
		if err == nil && len(values) == 4 {
			return fmt.Sprintf("ease.MustCubicBezier(%s, %s, %s, %s)",
				float(values[0]), float(values[1]), float(values[2]), float(values[3])), nil
		}
	}
	return "", fmt.Errorf("unsupported timing function %q", value)
}

func (g *generator) addPath(commands []PathCommand) int {
	g.paths = append(g.paths, commands)
	return len(g.paths) - 1
}

func (g *generator) emit() ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by arasy svg; DO NOT EDIT.\n\npackage %s\n\n", g.options.Package)
	out.WriteString(`import (
	"math"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/render/vector"
)

`)
	fmt.Fprintf(&out, "type %sResources struct {\n", g.options.Name)
	for _, font := range g.fonts {
		fmt.Fprintf(&out, "\t%s render.FontID // manifest font %q\n", font.field, font.manifest)
	}
	out.WriteString("}\n\n")
	fmt.Fprintf(&out, `// %sWorkspace retains generated command and vector mesh buffers between
// sequential frames. The returned commands remain valid only until the next
// call to Append on the same workspace. A workspace must not be used
// concurrently.
type %sWorkspace struct {
	commands []render.Command
	fills [%d]vector.FillWorkspace
	strokes [%d]vector.StrokeWorkspace
}

`, g.options.Name, g.options.Name, len(g.nodes), len(g.nodes))
	g.emitRuntime(&out)
	for index, path := range g.paths {
		g.emitPath(&out, index, path)
	}
	fmt.Fprintf(&out, "var %sNodes = []svgNode{\n", unexportedIdentifier(g.options.Name))
	for _, node := range g.nodes {
		g.emitNode(&out, node)
	}
	out.WriteString("}\n\n")
	fmt.Fprintf(&out, `// Append renders the SVG presentation while reusing buffers retained by w.
func (w *%sWorkspace) Append(timeline anim.Timeline, viewport math2d.Rect, resources %sResources) []render.Command {
	if w == nil {
		return Append%s(nil, timeline, viewport, resources)
	}
	w.commands = append%s(w.commands[:0], timeline, viewport, resources, w.fills[:], w.strokes[:])
	return w.commands
}

// Append%s appends the SVG presentation at timeline time into dst.
func Append%s(dst []render.Command, timeline anim.Timeline, viewport math2d.Rect, resources %sResources) []render.Command {
	return append%s(dst, timeline, viewport, resources, nil, nil)
}

func append%s(dst []render.Command, timeline anim.Timeline, viewport math2d.Rect, resources %sResources, fillWorkspaces []vector.FillWorkspace, strokeWorkspaces []vector.StrokeWorkspace) []render.Command {
	var builder render.Builder
	builder.Reset(dst)
	scale := math.Min(viewport.Width()/%s, viewport.Height()/%s)
	offset := math2d.Vec2{
		X: viewport.Min.X + (viewport.Width()-%s*scale)*0.5 - %s*scale,
		Y: viewport.Min.Y + (viewport.Height()-%s*scale)*0.5 - %s*scale,
	}
	builder.Push(render.Group{Transform: math2d.Translate(offset).Mul(math2d.Scale(math2d.Vec2{X: scale, Y: scale})), Opacity: 1})
	appendSVGNode(&builder, %sNodes, 0, timeline, resources.font, fillWorkspaces, strokeWorkspaces)
	builder.Pop()
	return builder.Commands()
}
`, g.options.Name, g.options.Name, g.options.Name, g.options.Name,
		g.options.Name, g.options.Name, g.options.Name, g.options.Name,
		g.options.Name, g.options.Name,
		float(g.document.ViewBox.Width()), float(g.document.ViewBox.Height()),
		float(g.document.ViewBox.Width()), float(g.document.ViewBox.Min.X),
		float(g.document.ViewBox.Height()), float(g.document.ViewBox.Min.Y),
		unexportedIdentifier(g.options.Name))
	// Resources may have multiple fonts; emit a resolver method on the generated type.
	fmt.Fprintf(&out, "\nfunc (r %sResources) font(field uint16) render.FontID {\n\tswitch field {\n", g.options.Name)
	for index, font := range g.fonts {
		fmt.Fprintf(&out, "\tcase %d:\n\t\treturn r.%s\n", index+1, font.field)
	}
	out.WriteString("\tdefault:\n\t\treturn 0\n\t}\n}\n")
	return out.Bytes(), nil
}

func (g *generator) emitRuntime(out *bytes.Buffer) {
	out.WriteString(`
type svgScalarStop struct{ progress, value float64 }
type svgAffineStop struct {
	progress float64
	value math2d.Affine
}
type svgAnimation struct {
	delay, duration float64
	curve ease.Curve
	opacity, dashOffset, distance []svgScalarStop
	transform []svgAffineStop
}
type svgNode struct {
	kind uint8
	children []uint16
	path, motionPath int16
	x, y, x2, y2, rx, ry, width, height float64
	text string
	fill, stroke render.Color
	strokeWidth, fontSize, spacing, opacity, pathLength float64
	font uint16
	align render.TextAlign
	transform math2d.Affine
	origin math2d.Vec2
	lineCap vector.LineCap
	lineJoin vector.LineJoin
	animation svgAnimation
}

func svgScalar(t anim.Timeline, animation svgAnimation, stops []svgScalarStop, fallback float64) float64 {
	if len(stops) == 0 { return fallback }
	elapsed := t.Elapsed()
	first := animation.delay + stops[0].progress*animation.duration
	if elapsed <= first { return stops[0].value }
	for index := 1; index < len(stops); index++ {
		end := animation.delay + stops[index].progress*animation.duration
		if elapsed <= end {
			start := animation.delay + stops[index-1].progress*animation.duration
			return t.Track(start, end, animation.curve).Lerp(stops[index-1].value, stops[index].value)
		}
	}
	return stops[len(stops)-1].value
}

func svgAffine(t anim.Timeline, animation svgAnimation, fallback math2d.Affine) math2d.Affine {
	stops := animation.transform
	if len(stops) == 0 { return fallback }
	elapsed := t.Elapsed()
	if elapsed <= animation.delay+stops[0].progress*animation.duration { return stops[0].value }
	for index := 1; index < len(stops); index++ {
		end := animation.delay + stops[index].progress*animation.duration
		if elapsed <= end {
			start := animation.delay + stops[index-1].progress*animation.duration
			p := t.Track(start, end, animation.curve).Value()
			a, b := stops[index-1].value, stops[index].value
			return math2d.Affine{
				A: a.A+(b.A-a.A)*p, B: a.B+(b.B-a.B)*p,
				C: a.C+(b.C-a.C)*p, D: a.D+(b.D-a.D)*p,
				E: a.E+(b.E-a.E)*p, F: a.F+(b.F-a.F)*p,
			}
		}
	}
	return stops[len(stops)-1].value
}

func appendSVGNode(
	builder *render.Builder,
	nodes []svgNode,
	index int,
	timeline anim.Timeline,
	font func(uint16) render.FontID,
	fillWorkspaces []vector.FillWorkspace,
	strokeWorkspaces []vector.StrokeWorkspace,
) {
	node := nodes[index]
	opacity := node.opacity
	if len(node.animation.opacity) != 0 {
		// CSS animations replace the underlying presentation attribute while
		// active. Multiplying would keep opacity="0" elements invisible.
		opacity = svgScalar(timeline, node.animation, node.animation.opacity, node.opacity)
	}
	if opacity <= 0 {
		return
	}
	transform := svgAffine(timeline, node.animation, node.transform)
	if node.origin != (math2d.Vec2{}) && len(node.animation.transform) != 0 {
		transform = transform.About(node.origin)
	}
	if node.motionPath >= 0 {
		progress := svgScalar(timeline, node.animation, node.animation.distance, 0)
		point, _, err := svgPaths[node.motionPath].PointAt(progress, 0.25)
		if err != nil { panic(err) }
		transform = math2d.Translate(point).Mul(transform)
	}
	builder.Push(render.Group{Transform: transform, Opacity: opacity})
	base := render.BaseCommand{}
	switch node.kind {
	case 1:
		builder.Add(render.DrawRect{BaseCommand: base, Rect: math2d.NewRect(math2d.Vec2{X: node.x, Y: node.y}, math2d.Vec2{X: node.width, Y: node.height}), Fill: node.fill, Stroke: node.stroke, StrokeWidth: node.strokeWidth, CornerRadius: node.rx})
	case 2:
		builder.Add(render.DrawCircle{BaseCommand: base, Center: math2d.Vec2{X: node.x, Y: node.y}, Radius: node.rx, Fill: node.fill, Stroke: node.stroke, StrokeWidth: node.strokeWidth})
	case 3:
		builder.Add(render.DrawEllipse{BaseCommand: base, Center: math2d.Vec2{X: node.x, Y: node.y}, RadiusX: node.rx, RadiusY: node.ry, Fill: node.fill, Stroke: node.stroke, StrokeWidth: node.strokeWidth})
	case 4:
		builder.Add(render.DrawLine{BaseCommand: base, From: math2d.Vec2{X: node.x, Y: node.y}, To: math2d.Vec2{X: node.x2, Y: node.y2}, Width: node.strokeWidth, Color: node.stroke})
	case 5:
		if node.fill.A != 0 {
			var commands []render.Command
			var err error
			if fillWorkspaces == nil {
				commands, err = svgPaths[node.path].AppendFill(nil, base, vector.FillStyle{Color: node.fill, Tolerance: 0.25})
			} else {
				commands, err = svgPaths[node.path].AppendFillWithWorkspace(nil, base, vector.FillStyle{Color: node.fill, Tolerance: 0.25}, &fillWorkspaces[index])
			}
			if err != nil { panic(err) }
			for _, command := range commands { builder.Add(command) }
		}
		if node.stroke.A != 0 && node.strokeWidth > 0 {
			trimEnd := 1.0
			if len(node.animation.dashOffset) != 0 && node.pathLength > 0 {
				trimEnd = 1-svgScalar(timeline, node.animation, node.animation.dashOffset, node.pathLength)/node.pathLength
			}
			var commands []render.Command
			var err error
			if strokeWorkspaces == nil {
				commands, err = svgPaths[node.path].AppendStroke(nil, base, vector.StrokeStyle{Color: node.stroke, Width: node.strokeWidth, LineCap: node.lineCap, LineJoin: node.lineJoin, TrimEnd: trimEnd, Tolerance: 0.25})
			} else {
				commands, err = svgPaths[node.path].AppendStrokeWithWorkspace(nil, base, vector.StrokeStyle{Color: node.stroke, Width: node.strokeWidth, LineCap: node.lineCap, LineJoin: node.lineJoin, TrimEnd: trimEnd, Tolerance: 0.25}, &strokeWorkspaces[index])
			}
			if err != nil { panic(err) }
			for _, command := range commands { builder.Add(command) }
		}
	case 6:
		builder.Add(render.DrawText{BaseCommand: base, Position: math2d.Vec2{X: node.x, Y: node.y}, Text: node.text, Color: node.fill, Size: node.fontSize, Font: font(node.font), LetterSpacing: node.spacing, Align: node.align})
	}
	for _, child := range node.children { appendSVGNode(builder, nodes, int(child), timeline, font, fillWorkspaces, strokeWorkspaces) }
	builder.Pop()
}

`)
	// All generated node sets in one file share this deterministic path table.
	out.WriteString("var svgPaths = []vector.Path{\n")
	for index := range g.paths {
		fmt.Fprintf(out, "\tsvgPath%d,\n", index)
	}
	out.WriteString("}\n\n")
}

func (g *generator) emitPath(out *bytes.Buffer, index int, commands []PathCommand) {
	fmt.Fprintf(out, "var svgPath%d = func() vector.Path {\n\tvar builder vector.Builder\n", index)
	for _, command := range commands {
		switch command.Verb {
		case PathMove:
			fmt.Fprintf(out, "\tbuilder.MoveTo(math2d.Vec2{X: %s, Y: %s})\n", float(command.P0.X), float(command.P0.Y))
		case PathLine:
			fmt.Fprintf(out, "\tbuilder.LineTo(math2d.Vec2{X: %s, Y: %s})\n", float(command.P0.X), float(command.P0.Y))
		case PathQuad:
			fmt.Fprintf(out, "\tbuilder.QuadTo(math2d.Vec2{X: %s, Y: %s}, math2d.Vec2{X: %s, Y: %s})\n",
				float(command.P0.X), float(command.P0.Y), float(command.P1.X), float(command.P1.Y))
		case PathCubic:
			fmt.Fprintf(out, "\tbuilder.CubicTo(math2d.Vec2{X: %s, Y: %s}, math2d.Vec2{X: %s, Y: %s}, math2d.Vec2{X: %s, Y: %s})\n",
				float(command.P0.X), float(command.P0.Y), float(command.P1.X), float(command.P1.Y), float(command.P2.X), float(command.P2.Y))
		case PathClose:
			out.WriteString("\tbuilder.Close()\n")
		}
	}
	out.WriteString("\tpath, err := builder.Build()\n\tif err != nil { panic(err) }\n\treturn path\n}()\n\n")
}

func (g *generator) emitNode(out *bytes.Buffer, node genNode) {
	kind := map[string]int{"rect": 1, "circle": 2, "ellipse": 3, "line": 4, "polyline": 5, "polygon": 5, "path": 5, "text": 6, "tspan": 6}[node.kind]
	fmt.Fprintf(out, "\t{kind:%d, path:%d, motionPath:%d, x:%s, y:%s, x2:%s, y2:%s, rx:%s, ry:%s, width:%s, height:%s, text:%q, fill:%s, stroke:%s, strokeWidth:%s, fontSize:%s, spacing:%s, opacity:%s, pathLength:%s, font:%d, align:%s, transform:%s, origin:math2d.Vec2{X:%s,Y:%s}, lineCap:%s, lineJoin:%s",
		kind, node.path, node.motionPath, float(node.x), float(node.y), float(node.x2), float(node.y2),
		float(node.rx), float(node.ry), float(node.width), float(node.height), node.text,
		colorExpression(node.fill), colorExpression(node.stroke), float(node.strokeW), float(node.fontSize),
		float(node.spacing), float(node.opacity), float(node.pathLength), g.fontIndex(node.fontField),
		alignExpression(node.align), affineExpression(node.transform), float(node.origin.X), float(node.origin.Y),
		node.lineCap, node.lineJoin)
	if len(node.children) != 0 {
		out.WriteString(", children: []uint16{")
		for _, child := range node.children {
			fmt.Fprintf(out, "%d,", child)
		}
		out.WriteString("}")
	}
	if node.animation.duration != 0 {
		fmt.Fprintf(out, ", animation:svgAnimation{delay:%s,duration:%s,curve:%s",
			float(node.animation.delay), float(node.animation.duration), node.animation.curve)
		emitScalarStops(out, "opacity", node.animation.opacity)
		emitScalarStops(out, "dashOffset", node.animation.dashOffset)
		emitScalarStops(out, "distance", node.animation.distance)
		if len(node.animation.transform) != 0 {
			out.WriteString(",transform:[]svgAffineStop{")
			for _, stop := range node.animation.transform {
				fmt.Fprintf(out, "{progress:%s,value:%s},", float(stop.progress), affineExpression(stop.value))
			}
			out.WriteString("}")
		}
		out.WriteString("}")
	}
	out.WriteString("},\n")
}

func emitScalarStops(out *bytes.Buffer, name string, stops []scalarStop) {
	if len(stops) == 0 {
		return
	}
	fmt.Fprintf(out, ",%s:[]svgScalarStop{", name)
	for _, stop := range stops {
		fmt.Fprintf(out, "{progress:%s,value:%s},", float(stop.progress), float(stop.value))
	}
	out.WriteString("}")
}

func (g *generator) fontIndex(field string) int {
	for index, font := range g.fonts {
		if font.field == field {
			return index + 1
		}
	}
	return 0
}

func parseColor(value string) (render.Color, error) {
	if value == "" || value == "none" {
		return render.Color{}, nil
	}
	return render.ColorFromHex(value)
}

func colorExpression(color render.Color) string {
	return fmt.Sprintf("render.RGBA(%s,%s,%s,%s)", float(color.R), float(color.G), float(color.B), float(color.A))
}

func affineExpression(value math2d.Affine) string {
	return fmt.Sprintf("math2d.Affine{A:%s,B:%s,C:%s,D:%s,E:%s,F:%s}",
		float(value.A), float(value.B), float(value.C), float(value.D), float(value.E), float(value.F))
}

func alignExpression(value string) string {
	switch value {
	case "middle":
		return "render.AlignCenter"
	case "end":
		return "render.AlignRight"
	default:
		return "render.AlignLeft"
	}
}

func lineCapName(value string) string {
	switch value {
	case "round":
		return "vector.CapRound"
	case "square":
		return "vector.CapSquare"
	default:
		return "vector.CapButt"
	}
}

func lineJoinName(value string) string {
	switch value {
	case "round":
		return "vector.JoinRound"
	case "bevel":
		return "vector.JoinBevel"
	default:
		return "vector.JoinMiter"
	}
}

func nodeBounds(node genNode) math2d.Rect {
	switch node.kind {
	case "rect":
		return math2d.NewRect(math2d.Vec2{X: node.x, Y: node.y}, math2d.Vec2{X: node.width, Y: node.height})
	case "circle", "ellipse":
		return math2d.NewRect(math2d.Vec2{X: node.x - node.rx, Y: node.y - node.ry}, math2d.Vec2{X: 2 * node.rx, Y: 2 * node.ry})
	default:
		return math2d.Rect{}
	}
}

func pointsPath(value string, close bool) ([]PathCommand, error) {
	values, err := parseNumberList(value)
	if err != nil || len(values) < 4 || len(values)%2 != 0 {
		return nil, fmt.Errorf("expected at least two coordinate pairs")
	}
	commands := []PathCommand{{Verb: PathMove, P0: math2d.Vec2{X: values[0], Y: values[1]}}}
	for index := 2; index < len(values); index += 2 {
		commands = append(commands, PathCommand{Verb: PathLine, P0: math2d.Vec2{X: values[index], Y: values[index+1]}})
	}
	if close {
		commands = append(commands, PathCommand{Verb: PathClose})
	}
	return commands, nil
}

func pathFunction(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "path(") || !strings.HasSuffix(value, ")") {
		return "", false
	}
	value = strings.TrimSpace(value[5 : len(value)-1])
	if len(value) < 2 || value[0] != value[len(value)-1] || value[0] != '\'' && value[0] != '"' {
		return "", false
	}
	return value[1 : len(value)-1], true
}

func numberAttribute(element *Element, key string, fallback float64) (float64, error) {
	value := element.Attributes[key]
	if value == "" {
		return fallback, nil
	}
	number, err := cssNumber(value)
	if err != nil {
		return 0, lineError(element, key, err)
	}
	return number, nil
}

func numberDefault(value string, fallback float64) float64 {
	if value == "" {
		return fallback
	}
	number, err := cssNumber(value)
	if err != nil {
		return fallback
	}
	return number
}

func cssNumber(value string) (float64, error) {
	value = strings.TrimSpace(value)
	for _, suffix := range []string{"px", "%"} {
		value = strings.TrimSuffix(value, suffix)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("invalid number %q", value)
	}
	return number, nil
}

func percentNumber(value string) float64 {
	number, err := cssNumber(value)
	if err != nil {
		return 0
	}
	if strings.HasSuffix(strings.TrimSpace(value), "%") {
		return number / 100
	}
	return number
}

func cssSeconds(value string) (float64, error) {
	value = strings.TrimSpace(value)
	scale := 1.0
	switch {
	case strings.HasSuffix(value, "ms"):
		scale, value = 0.001, strings.TrimSuffix(value, "ms")
	case strings.HasSuffix(value, "s"):
		value = strings.TrimSuffix(value, "s")
	default:
		return 0, fmt.Errorf("duration %q requires s or ms", value)
	}
	number, err := strconv.ParseFloat(value, 64)
	return number * scale, err
}

func lineError(element *Element, subject string, err error) error {
	return fmt.Errorf("svg:%d: %s.%s: %w", element.Line, element.Name, subject, err)
}

func float(value float64) string {
	if value == 0 {
		return "0"
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if !(char == '_' || unicode.IsLetter(char) || index > 0 && unicode.IsDigit(char)) {
			return false
		}
	}
	return true
}

func validExportedIdentifier(value string) bool {
	return validIdentifier(value) && unicode.IsUpper([]rune(value)[0])
}

func exportedIdentifier(value string) string {
	var result []rune
	upper := true
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			upper = true
			continue
		}
		if upper {
			char = unicode.ToUpper(char)
			upper = false
		}
		result = append(result, char)
	}
	if len(result) == 0 || !unicode.IsLetter(result[0]) {
		result = append([]rune{'F'}, result...)
	}
	return string(result)
}

func unexportedIdentifier(value string) string {
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

package svggen

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var acceptedProperties = map[string]bool{
	"opacity": true, "fill": true, "stroke": true, "stroke-width": true,
	"stroke-linecap": true, "stroke-linejoin": true, "stroke-miterlimit": true,
	"font-family": true, "font-size": true, "font-weight": true,
	"text-anchor": true, "letter-spacing": true, "transform": true,
	"transform-box": true, "transform-origin": true,
	"animation": true, "animation-name": true, "animation-duration": true,
	"animation-delay": true, "animation-iteration-count": true,
	"animation-direction": true, "animation-fill-mode": true,
	"animation-timing-function": true, "stroke-dasharray": true,
	"stroke-dashoffset": true, "offset-path": true, "offset-distance": true,
	"filter": true,
}

func parseCSS(source string, strict bool) (Stylesheet, error) {
	source = stripCSSComments(source)
	stylesheet := Stylesheet{Keyframes: make(map[string][]Keyframe)}
	ruleCount := 0
	for strings.TrimSpace(source) != "" {
		header, body, rest, err := takeCSSBlock(source)
		if err != nil {
			return stylesheet, err
		}
		source = rest
		header = strings.TrimSpace(header)
		ruleCount++
		if ruleCount > MaxCSSRules {
			return stylesheet, fmt.Errorf("svg: CSS rule limit %d exceeded", MaxCSSRules)
		}
		switch {
		case strings.HasPrefix(strings.ToLower(header), "@keyframes "):
			name := strings.TrimSpace(header[len("@keyframes "):])
			frames, err := parseKeyframes(body, strict, &ruleCount)
			if err != nil {
				return stylesheet, fmt.Errorf("svg: @keyframes %s: %w", name, err)
			}
			stylesheet.Keyframes[name] = frames
		case strings.EqualFold(header, "@font-face"):
			declarations, err := parseDeclarations(body, strict)
			if err != nil {
				return stylesheet, err
			}
			sourceValue := declarations["src"]
			start := strings.Index(sourceValue, "data:font/ttf;base64,")
			if start < 0 {
				return stylesheet, fmt.Errorf("svg: @font-face only accepts embedded TTF data URLs")
			}
			payload := sourceValue[start+len("data:font/ttf;base64,"):]
			if end := strings.IndexAny(payload, ") \t\r\n"); end >= 0 {
				payload = payload[:end]
			}
			decoded, err := base64.StdEncoding.DecodeString(payload)
			if err != nil || len(decoded) > MaxDataURLBytes {
				return stylesheet, fmt.Errorf("svg: invalid or oversized @font-face payload")
			}
		case strings.HasPrefix(header, "@"):
			return stylesheet, fmt.Errorf("svg: CSS at-rule %q is outside the supported subset", header)
		default:
			if !validSelector(header) {
				return stylesheet, fmt.Errorf("svg: CSS selector %q is outside the supported subset", header)
			}
			declarations, err := parseDeclarations(body, strict)
			if err != nil {
				return stylesheet, fmt.Errorf("svg: selector %s: %w", header, err)
			}
			stylesheet.Rules = append(stylesheet.Rules, Rule{Selector: header, Declarations: declarations})
		}
	}
	return stylesheet, nil
}

func takeCSSBlock(source string) (header, body, rest string, err error) {
	open := strings.IndexByte(source, '{')
	if open < 0 {
		return "", "", "", fmt.Errorf("svg: malformed CSS block")
	}
	depth := 1
	for index := open + 1; index < len(source); index++ {
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[:open], source[open+1 : index], source[index+1:], nil
			}
		}
	}
	return "", "", "", fmt.Errorf("svg: unterminated CSS block")
}

func parseKeyframes(source string, strict bool, count *int) ([]Keyframe, error) {
	var frames []Keyframe
	for strings.TrimSpace(source) != "" {
		header, body, rest, err := takeCSSBlock(source)
		if err != nil {
			return nil, err
		}
		source = rest
		(*count)++
		if *count > MaxCSSRules {
			return nil, fmt.Errorf("CSS rule limit exceeded")
		}
		for _, selector := range strings.Split(header, ",") {
			value := strings.TrimSpace(selector)
			progress := 0.0
			switch value {
			case "from":
			case "to":
				progress = 1
			default:
				if !strings.HasSuffix(value, "%") {
					return nil, fmt.Errorf("invalid keyframe selector %q", value)
				}
				number, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
				if err != nil || number < 0 || number > 100 {
					return nil, fmt.Errorf("invalid keyframe selector %q", value)
				}
				progress = number / 100
			}
			declarations, err := parseDeclarations(body, strict)
			if err != nil {
				return nil, err
			}
			frames = append(frames, Keyframe{Progress: progress, Declarations: declarations})
		}
	}
	sort.SliceStable(frames, func(left, right int) bool { return frames[left].Progress < frames[right].Progress })
	return frames, nil
}

func parseDeclarations(source string, strict bool) (map[string]string, error) {
	result := make(map[string]string)
	for _, raw := range splitDeclarations(source) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed declaration %q", raw)
		}
		property, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		lower := strings.ToLower(value)
		if strings.Contains(lower, "url(http") || strings.Contains(lower, "javascript:") ||
			strings.Contains(lower, "expression(") {
			return nil, fmt.Errorf("%s contains a forbidden resource", property)
		}
		if property == "src" {
			result[property] = value
			continue
		}
		if !acceptedProperties[property] {
			if strict {
				return nil, fmt.Errorf("property %s is outside the supported subset", property)
			}
			continue
		}
		result[property] = value
	}
	return result, nil
}

func splitDeclarations(source string) []string {
	var result []string
	start, depth := 0, 0
	var quote rune
	for index, char := range source {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
		case char == '(':
			depth++
		case char == ')' && depth > 0:
			depth--
		case char == ';' && depth == 0:
			result = append(result, source[start:index])
			start = index + 1
		}
	}
	return append(result, source[start:])
}

func validSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" || strings.ContainsAny(selector, " +~,:[]*") {
		return false
	}
	if strings.Count(selector, ">") == 1 {
		parts := strings.Split(selector, ">")
		return validSimpleSelector(parts[0]) && validSimpleSelector(parts[1])
	}
	if strings.Contains(selector, ">") {
		return false
	}
	return validSimpleSelector(selector)
}

func validSimpleSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return false
	}
	if selector[0] == '.' || selector[0] == '#' {
		selector = selector[1:]
	}
	for _, char := range selector {
		if !(char == '-' || char == '_' || char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return selector != ""
}

func applyStyles(root *Element, stylesheet Stylesheet) {
	var walk func(*Element, *Element)
	walk = func(element, parent *Element) {
		type matched struct {
			rule        Rule
			specificity int
			order       int
		}
		var matches []matched
		for order, rule := range stylesheet.Rules {
			specificity := selectorMatch(rule.Selector, element, parent)
			if specificity >= 0 {
				matches = append(matches, matched{rule: rule, specificity: specificity, order: order})
			}
		}
		sort.SliceStable(matches, func(left, right int) bool {
			if matches[left].specificity == matches[right].specificity {
				return matches[left].order < matches[right].order
			}
			return matches[left].specificity < matches[right].specificity
		})
		for _, match := range matches {
			for property, value := range match.rule.Declarations {
				if _, explicitlySet := element.Attributes[property]; !explicitlySet {
					element.Attributes[property] = value
				}
			}
		}
		if inline := element.Attributes["style"]; inline != "" {
			if declarations, err := parseDeclarations(inline, true); err == nil {
				for property, value := range declarations {
					element.Attributes[property] = value
				}
			}
		}
		for _, child := range element.Children {
			walk(child, element)
		}
	}
	walk(root, nil)
}

func selectorMatch(selector string, element, parent *Element) int {
	if strings.Contains(selector, ">") {
		parts := strings.Split(selector, ">")
		if parent == nil {
			return -1
		}
		left := selectorMatch(strings.TrimSpace(parts[0]), parent, nil)
		right := selectorMatch(strings.TrimSpace(parts[1]), element, nil)
		if left < 0 || right < 0 {
			return -1
		}
		return left + right
	}
	switch selector[0] {
	case '#':
		if element.Attributes["id"] == selector[1:] {
			return 100
		}
	case '.':
		for _, class := range strings.Fields(element.Attributes["class"]) {
			if class == selector[1:] {
				return 10
			}
		}
	default:
		if element.Name == selector {
			return 1
		}
	}
	return -1
}

func stripCSSComments(source string) string {
	for {
		start := strings.Index(source, "/*")
		if start < 0 {
			return source
		}
		end := strings.Index(source[start+2:], "*/")
		if end < 0 {
			return source[:start]
		}
		source = source[:start] + source[start+2+end+2:]
	}
}

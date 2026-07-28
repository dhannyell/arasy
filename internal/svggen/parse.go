package svggen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

var acceptedElements = map[string]bool{
	"svg": true, "g": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true, "path": true,
	"text": true, "tspan": true, "style": true,
}

var forbiddenFragments = []string{
	"<!doctype", "<!entity", "<?xml-stylesheet", "<script", "javascript:",
	"@import", "url(http:", "url(https:", "url(//",
}

func Parse(data []byte, options Options) (*Document, error) {
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("svg: input exceeds %d bytes", MaxInputBytes)
	}
	lower := strings.ToLower(string(data))
	for _, fragment := range forbiddenFragments {
		if strings.Contains(lower, fragment) {
			return nil, fmt.Errorf("svg: forbidden external or executable content %q", fragment)
		}
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	document := &Document{Fonts: cloneMap(options.Fonts)}
	var stack []*Element
	elementCount := 0
	dataURLBytes := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, sourceError(data, decoder.InputOffset(), "XML", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			elementCount++
			if elementCount > MaxElements {
				return nil, fmt.Errorf("svg: element limit %d exceeded", MaxElements)
			}
			if len(stack)+1 > MaxDepth {
				return nil, fmt.Errorf("svg: DOM depth limit %d exceeded", MaxDepth)
			}
			name := token.Name.Local
			if token.Name.Space != "" && token.Name.Space != "http://www.w3.org/2000/svg" || !acceptedElements[name] {
				return nil, sourceError(data, decoder.InputOffset(), name, fmt.Errorf("element is outside the supported subset"))
			}
			if name == "style" && (len(stack) != 1 || stack[0].Name != "svg") {
				return nil, sourceError(data, decoder.InputOffset(), name, fmt.Errorf("style must be a direct child of svg"))
			}
			element := &Element{
				Name: name, Attributes: make(map[string]string),
				Line: lineAt(data, decoder.InputOffset()),
			}
			for _, attribute := range token.Attr {
				if attribute.Name.Space == "xmlns" || attribute.Name.Local == "xmlns" {
					continue
				}
				key := attribute.Name.Local
				if attribute.Name.Space != "" {
					key = attribute.Name.Space + ":" + key
				}
				value := strings.TrimSpace(attribute.Value)
				lowerKey := strings.ToLower(key)
				lowerValue := strings.ToLower(value)
				if strings.HasPrefix(lowerKey, "on") || lowerKey == "href" || lowerKey == "xlink:href" ||
					strings.Contains(lowerValue, "javascript:") || strings.Contains(lowerValue, "url(http") {
					return nil, sourceError(data, decoder.InputOffset(), name+"."+key, fmt.Errorf("external or executable resource is forbidden"))
				}
				if strings.HasPrefix(lowerValue, "data:") {
					dataURLBytes += len(value)
					if dataURLBytes > MaxDataURLBytes {
						return nil, fmt.Errorf("svg: data URL limit %d exceeded", MaxDataURLBytes)
					}
				}
				element.Attributes[key] = value
			}
			if len(stack) == 0 {
				if name != "svg" {
					return nil, fmt.Errorf("svg: root element must be svg")
				}
				document.Root = element
			} else {
				stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, element)
			}
			stack = append(stack, element)
		case xml.CharData:
			if len(stack) != 0 {
				stack[len(stack)-1].Text += string(token)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("svg: unexpected closing element %s", token.Name.Local)
			}
			stack = stack[:len(stack)-1]
		case xml.Directive:
			return nil, sourceError(data, decoder.InputOffset(), "directive", fmt.Errorf("XML directives are forbidden"))
		case xml.ProcInst:
			if strings.ToLower(token.Target) != "xml" {
				return nil, sourceError(data, decoder.InputOffset(), token.Target, fmt.Errorf("processing instructions are forbidden"))
			}
		}
	}
	if document.Root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("svg: incomplete document")
	}
	if err := validateDocument(document, options); err != nil {
		return nil, err
	}
	return document, nil
}

func cloneMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func sourceError(data []byte, offset int64, subject string, err error) error {
	return fmt.Errorf("svg:%d: %s: %w", lineAt(data, offset), subject, err)
}

func lineAt(data []byte, offset int64) int {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	return 1 + bytes.Count(data[:offset], []byte{'\n'})
}

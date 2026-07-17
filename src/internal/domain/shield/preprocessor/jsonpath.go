package preprocessor

import (
	"fmt"
	"strconv"
	"strings"
)

type jsonPathSegment struct {
	key     string
	isIndex bool
	index   int
}

// @sk-task 25-shield-preprocessors#T2.2: Implement JSONPath parser and walker (AC-003, AC-004, AC-005)
func parseJSONPath(path string) ([]jsonPathSegment, error) {
	if path == "" {
		return nil, fmt.Errorf("empty JSONPath")
	}
	parts := strings.Split(path, ".")
	segments := make([]jsonPathSegment, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("invalid JSONPath: empty segment in %q", path)
		}

		if strings.HasPrefix(p, "[") {
			// Bare bracket segment: [*], [0]
			inner := p[1 : len(p)-1]
			if inner == "*" {
				segments = append(segments, jsonPathSegment{key: "[*]"})
			} else {
				idx, err := strconv.Atoi(inner)
				if err != nil {
					return nil, fmt.Errorf("invalid array index %q in JSONPath %q", p, path)
				}
				segments = append(segments, jsonPathSegment{key: p, isIndex: true, index: idx})
			}
		} else if idx := strings.Index(p, "["); idx >= 0 {
			// Combined segment: items[*], items[0]
			keyPart := p[:idx]
			if keyPart != "" {
				segments = append(segments, jsonPathSegment{key: keyPart})
			}
			inner := p[idx+1 : len(p)-1]
			if inner == "*" {
				segments = append(segments, jsonPathSegment{key: "[*]"})
			} else {
				n, err := strconv.Atoi(inner)
				if err != nil {
					return nil, fmt.Errorf("invalid array index %q in JSONPath %q", p, path)
				}
				segments = append(segments, jsonPathSegment{key: "[" + inner + "]", isIndex: true, index: n})
			}
		} else {
			segments = append(segments, jsonPathSegment{key: p})
		}
	}
	return segments, nil
}

// @sk-task 25-shield-preprocessors#T2.2: Implement JSONPath walker with wildcard support (AC-003, AC-004, AC-005)
func walkAndMask(node interface{}, segments []jsonPathSegment, phPrefix string, phIdx *int, replacements map[string]string) interface{} {
	if len(segments) == 0 {
		return node
	}

	seg := segments[0]
	rest := segments[1:]

	switch v := node.(type) {
	case map[string]interface{}:
		val, ok := v[seg.key]
		if !ok {
			return v
		}
		if len(rest) == 0 {
			orig := fmt.Sprintf("%v", val)
			ph := fmt.Sprintf("[MASK_%s.%d]", phPrefix, *phIdx)
			replacements[orig] = ph
			*phIdx++
			v[seg.key] = ph
			return v
		}
		v[seg.key] = walkAndMask(val, rest, phPrefix, phIdx, replacements)
		return v

	case []interface{}:
		if seg.key == "[*]" {
			for i := range v {
				if len(rest) == 0 {
					orig := fmt.Sprintf("%v", v[i])
					ph := fmt.Sprintf("[MASK_%s.%d]", phPrefix, *phIdx)
					replacements[orig] = ph
					*phIdx++
					v[i] = ph
				} else {
					v[i] = walkAndMask(v[i], rest, phPrefix, phIdx, replacements)
				}
			}
			return v
		}
		if seg.isIndex {
			if seg.index < 0 || seg.index >= len(v) {
				return v
			}
			if len(rest) == 0 {
				orig := fmt.Sprintf("%v", v[seg.index])
				ph := fmt.Sprintf("[MASK_%s.%d]", phPrefix, *phIdx)
				replacements[orig] = ph
				*phIdx++
				v[seg.index] = ph
			} else {
				v[seg.index] = walkAndMask(v[seg.index], rest, phPrefix, phIdx, replacements)
			}
			return v
		}
		return v

	default:
		return node
	}
}

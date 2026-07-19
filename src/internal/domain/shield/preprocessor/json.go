package preprocessor

import (
	"encoding/json"
	"strings"
)

type JSONProcessor struct {
	name  string
	rules []Rule
}

func (p *JSONProcessor) Name() string { return p.name }

// @sk-task 25-shield-preprocessors#T2.2: Implement JSONProcessor.Process (AC-003, AC-004, AC-005)
func (p *JSONProcessor) Process(data string, namespace string) *ProcessResult {
	if len(p.rules) == 0 {
		return &ProcessResult{ModifiedText: data, Replacements: map[string]string{}}
	}

	// Build path -> mask mode map
	pathRules := make([]struct {
		segments []jsonPathSegment
		mask     string
	}, 0, len(p.rules))
	for _, r := range p.rules {
		if r.Path == "" {
			continue
		}
		segs, err := parseJSONPath(r.Path)
		if err != nil {
			continue
		}
		pathRules = append(pathRules, struct {
			segments []jsonPathSegment
			mask     string
		}{segs, string(r.Mask)})
	}
	if len(pathRules) == 0 {
		return &ProcessResult{ModifiedText: data, Replacements: map[string]string{}}
	}

	phPrefix := "json." + namespace
	var phIdx int
	replacements := make(map[string]string)

	// Find JSON blocks: {}-balanced or []-balanced, or inside ```json fences
	type jsonBlock struct {
		start, end int
	}
	var blocks []jsonBlock

	// First, handle ```json fences
	fenceStart := -1
	lines := strings.Split(data, "\n")
	for li, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```json" || trimmed == "```" {
			if fenceStart == -1 {
				fenceStart = li
			} else {
				// Calculate byte offsets
				fStart := 0
				for k := 0; k < fenceStart; k++ {
					fStart += len(lines[k]) + 1
				}
				fStart += len(lines[fenceStart]) + 1 // skip fence line itself + newline

				fEnd := 0
				for k := 0; k < li; k++ {
					fEnd += len(lines[k]) + 1
				}
				fEnd -= 1 // newline before closing fence, point to last char of content

				if fEnd > fStart {
					blocks = append(blocks, jsonBlock{fStart, fEnd})
				}
				fenceStart = -1
			}
		}
	}

	// Find top-level { or [ - balanced blocks outside fences
	depth := 0
	blockStart := -1

	// Mark fence byte ranges as "inFence"
	type fenceRange struct{ start, end int }
	var fenceRanges []fenceRange
	fenceStart = -1
	for li, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```json" || trimmed == "```" {
			lineStart := 0
			for k := 0; k < li; k++ {
				lineStart += len(lines[k]) + 1
			}
			if fenceStart == -1 {
				fenceStart = lineStart
			} else {
				lineEnd := lineStart + len(line)
				fenceRanges = append(fenceRanges, fenceRange{fenceStart, lineEnd})
				fenceStart = -1
			}
		}
	}

	isInFence := func(pos int) bool {
		for _, fr := range fenceRanges {
			if pos >= fr.start && pos <= fr.end {
				return true
			}
		}
		return false
	}

	for pos := 0; pos < len(data); pos++ {
		ch := data[pos]
		if isInFence(pos) {
			continue
		}
		if ch == '{' || ch == '[' {
			if depth == 0 {
				blockStart = pos
			}
			depth++
		} else if ch == '}' || ch == ']' {
			depth--
			if depth == 0 && blockStart != -1 {
				blocks = append(blocks, jsonBlock{blockStart, pos + 1})
				blockStart = -1
			}
		}
	}

	// Process blocks in reverse order
	for bi := len(blocks) - 1; bi >= 0; bi-- {
		b := blocks[bi]
		jsonStr := data[b.start:b.end]

		var parsed interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			continue
		}

		// Snapshot compact form before modifications
		origCompact, _ := json.Marshal(parsed)

		for _, pr := range pathRules {
			walkAndMask(parsed, pr.segments, phPrefix, &phIdx, replacements)
		}

		maskedBytes, err := json.Marshal(parsed)
		if err != nil {
			continue
		}

		// Only replace if the tree was actually modified
		if string(maskedBytes) != string(origCompact) {
			data = data[:b.start] + string(maskedBytes) + data[b.end:]
		}
	}

	rm := make(map[string]string)
	for k, v := range replacements {
		rm[v] = k
	}
	return &ProcessResult{ModifiedText: data, Replacements: rm}
}

package detector

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
)

var _ Detector = (*DictionaryDetector)(nil)

// @sk-task 24-shield-dictionaries#T2.1: Implement DictionaryDetector with exact match (AC-003)
type DictionaryDetector struct {
	dict *dictionary.Dictionary
}

func NewDictionaryDetector(dict *dictionary.Dictionary) *DictionaryDetector {
	return &DictionaryDetector{dict: dict}
}

func (d *DictionaryDetector) Dict() *dictionary.Dictionary {
	return d.dict
}

func (d *DictionaryDetector) Scan(ctx context.Context, text string) ([]DetectorResult, error) {
	if d.dict == nil || len(d.dict.AllValues()) == 0 {
		return nil, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch d.dict.MatchMode() {
	case dictionary.MatchModeExact:
		return d.scanExact(ctx, text), nil
	case dictionary.MatchModeContains:
		return d.scanContains(ctx, text), nil
	case dictionary.MatchModeRegex:
		return d.scanRegex(ctx, text)
	case dictionary.MatchModeFuzzy:
		return d.scanFuzzy(ctx, text), nil
	default:
		return nil, nil
	}
}

func (d *DictionaryDetector) scanExact(ctx context.Context, text string) []DetectorResult {
	entries := d.dict.AllValues()
	var results []DetectorResult
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return results
		default:
		}
		if entry == "" {
			continue
		}
		start := 0
		for {
			idx := strings.Index(text[start:], entry)
			if idx == -1 {
				break
			}
			pos := start + idx
			results = append(results, DetectorResult{
				DetectorType: "dictionary",
				Fragment:     entry,
				StartPos:     pos,
				EndPos:       pos + len(entry),
				Confidence:   1.0,
			})
			start = pos + 1
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartPos < results[j].StartPos
	})
	return results
}

func (d *DictionaryDetector) scanContains(ctx context.Context, text string) []DetectorResult {
	matcher := dictionary.BuildWordlistMatcher(d.dict.AllValues())
	matches := matcher.Match(text)

	results := make([]DetectorResult, 0, len(matches))
	for _, m := range matches {
		select {
		case <-ctx.Done():
			return results
		default:
		}
		results = append(results, DetectorResult{
			DetectorType: "dictionary",
			Fragment:     m.Pattern,
			StartPos:     m.Start,
			EndPos:       m.End,
			Confidence:   1.0,
		})
	}
	return results
}

// @sk-task 24-shield-dictionaries#T3.2: Implement regex mode (AC-005)
func (d *DictionaryDetector) scanRegex(ctx context.Context, text string) ([]DetectorResult, error) {
	var results []DetectorResult

	for _, entry := range d.dict.AllValues() {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}
		re, err := regexp.Compile(entry)
		if err != nil {
			continue
		}
		locs := re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			results = append(results, DetectorResult{
				DetectorType: "dictionary",
				Fragment:     text[loc[0]:loc[1]],
				StartPos:     loc[0],
				EndPos:       loc[1],
				Confidence:   1.0,
			})
		}
	}
	return results, nil
}

// @sk-task 24-shield-dictionaries#T3.3: Implement fuzzy mode with Levenshtein (AC-005)
func (d *DictionaryDetector) scanFuzzy(ctx context.Context, text string) []DetectorResult {
	entries := d.dict.AllValues()
	if len(entries) == 0 {
		return nil
	}

	words := strings.Fields(text)
	var results []DetectorResult

	pos := 0
	for _, word := range words {
		select {
		case <-ctx.Done():
			return results
		default:
		}
		idx := strings.Index(text[pos:], word)
		if idx == -1 {
			pos += len(word) + 1
			continue
		}
		start := pos + idx
		end := start + len(word)

		for _, entry := range entries {
			dist := levenshtein(word, entry)
			maxLen := len(word)
			if len(entry) > maxLen {
				maxLen = len(entry)
			}
			var similarity float64
			if maxLen > 0 {
				similarity = 1.0 - float64(dist)/float64(maxLen)
			}
			if similarity >= 0.8 {
				results = append(results, DetectorResult{
					DetectorType: "dictionary",
					Fragment:     word,
					StartPos:     start,
					EndPos:       end,
					Confidence:   similarity,
				})
				break
			}
		}
		pos = end
	}
	return results
}

func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

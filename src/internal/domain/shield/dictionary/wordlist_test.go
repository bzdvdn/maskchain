package dictionary

import (
	"math/rand"
	"strings"
	"testing"
)

// @sk-test 24-shield-dictionaries#T6.1: TestWordlistMatcherBasic (AC-008)
func TestWordlistMatcher_Basic(t *testing.T) {
	m := BuildWordlistMatcher([]string{"foo", "bar"})
	matches := m.Match("foo and bar and foo")

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %v", len(matches), matches)
	}

	if matches[0].Pattern != "foo" || matches[0].Start != 0 || matches[0].End != 3 {
		t.Errorf("unexpected first match: %+v", matches[0])
	}
	if matches[1].Pattern != "bar" || matches[1].Start != 8 || matches[1].End != 11 {
		t.Errorf("unexpected second match: %+v", matches[1])
	}
	if matches[2].Pattern != "foo" || matches[2].Start != 16 || matches[2].End != 19 {
		t.Errorf("unexpected third match: %+v", matches[2])
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestWordlistMatcherNoMatch (AC-008)
func TestWordlistMatcher_NoMatch(t *testing.T) {
	m := BuildWordlistMatcher([]string{"xyz"})
	matches := m.Match("hello world")
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %v", matches)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestWordlistMatcherEmpty (AC-008)
func TestWordlistMatcher_Empty(t *testing.T) {
	m := BuildWordlistMatcher(nil)
	matches := m.Match("hello")
	if len(matches) != 0 {
		t.Errorf("expected no matches for empty matcher, got %v", matches)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestWordlistMatcherOverlap (AC-008)
func TestWordlistMatcher_Overlap(t *testing.T) {
	m := BuildWordlistMatcher([]string{"he", "hello", "world"})
	matches := m.Match("hello world")

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d: %v", len(matches), matches)
	}
}

// @sk-test 24-shield-dictionaries#T6.1: TestWordlistMatcherSubstring (AC-008)
func TestWordlistMatcher_Substring(t *testing.T) {
	m := BuildWordlistMatcher([]string{"example.com"})
	matches := m.Match("visit sub.example.com for test")

	if len(matches) == 0 {
		t.Fatal("expected at least 1 match")
	}
	found := false
	for _, match := range matches {
		if match.Pattern == "example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'example.com' in matches: %v", matches)
	}
}

// @sk-bench 24-shield-dictionaries: 1000 terms Aho-Corasick benchmark
func BenchmarkAhoCorasick_1000Terms(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	terms := make([]string, 1000)
	termSet := make(map[string]bool)
	for i := range terms {
		// generate unique 8-12 char terms
		term := randomWord(rng, 8+rng.Intn(4))
		if termSet[term] {
			i--
			continue
		}
		termSet[term] = true
		terms[i] = term
	}

	m := BuildWordlistMatcher(terms)
	_ = m // built once; benchmark matches below

	// 10KB text with ~100 embedded terms
	var sb strings.Builder
	for sb.Len() < 10_000 {
		term := terms[rng.Intn(len(terms))]
		sb.WriteString(term)
		sb.WriteString(" and some filler text. ")
		// inject non-matching text too
		sb.WriteString(randomWord(rng, 4) + " " + randomWord(rng, 6) + ". ")
	}
	text := sb.String()

	b.ResetTimer()
	var total int
	for i := 0; i < b.N; i++ {
		matches := m.Match(text)
		total += len(matches)
	}
	b.StopTimer()
	b.ReportMetric(float64(total)/float64(b.N), "matches/op")
}

func randomWord(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

// run as: go test -bench=BenchmarkAhoCorasick_1000Terms -benchmem -count=1 -run=^$

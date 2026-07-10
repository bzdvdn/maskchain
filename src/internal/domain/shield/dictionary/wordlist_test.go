package dictionary

import (
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

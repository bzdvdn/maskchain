package detector

import (
	"testing"
	"unicode/utf8"
)

func FuzzValidLuhn(f *testing.F) {
	seeds := []string{
		"4532015112830366",
		"",
		"0",
		"4999999876543210",
		"abc",
		"1234",
		"4111111111111111",
		"5500000000000004",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		result := validLuhn(s)
		// Must never panic
		// If result is true, input must be non-empty and all digits
		if result {
			if len(s) == 0 {
				t.Errorf("validLuhn returned true for empty string")
			}
			for _, c := range s {
				if c < '0' || c > '9' {
					t.Errorf("validLuhn returned true for non-digit input %q", s)
				}
			}
		}
	})
}

func FuzzLevenshtein(f *testing.F) {
	seeds := []struct {
		a, b string
	}{
		{"", ""},
		{"a", ""},
		{"", "b"},
		{"hello", "world"},
		{"abc", "abc"},
		{"abc", "ab"},
	}
	for _, s := range seeds {
		f.Add(s.a, s.b)
	}

	f.Fuzz(func(t *testing.T, a, b string) {
		dist := levenshtein(a, b)

		if dist < 0 {
			t.Errorf("levenshtein(%q, %q) = %d, expected >= 0", a, b, dist)
		}

		// Distance must not exceed the length of the longer string
		maxLen := len(a)
		if len(b) > maxLen {
			maxLen = len(b)
		}
		if dist > maxLen && maxLen > 0 {
			t.Errorf("levenshtein(%q, %q) = %d, expected <= %d", a, b, dist, maxLen)
		}

		// Symmetry: levenshtein(a,b) == levenshtein(b,a)
		rev := levenshtein(b, a)
		if dist != rev {
			t.Errorf("levenshtein not symmetric: (%q, %q) = %d, but reverse = %d", a, b, dist, rev)
		}

		// Must return valid UTF-8 strings unchanged (no crash on invalid UTF-8)
		if utf8.ValidString(a) && utf8.ValidString(b) {
			// Just verify no panic for valid strings
			_ = levenshtein(a, b)
		}
	})
}

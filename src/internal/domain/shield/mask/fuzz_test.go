package mask

import (
	"testing"
)

func FuzzBase62Encode(f *testing.F) {
	seeds := [][]byte{
		{},
		{0},
		{255},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{1, 2, 3, 4, 5, 6, 7, 8},
		{255, 255, 255, 255, 255, 255, 255, 255},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		result := base62Encode(src)
		if result == "" {
			t.Errorf("base62Encode(%v) returned empty string", src)
		}
		// All characters must be valid base62
		for _, c := range result {
			if !isBase62Char(c) {
				t.Errorf("base62Encode(%v) = %q contains invalid char %c", src, result, c)
			}
		}
		// Deterministic: same input must produce same output
		result2 := base62Encode(src)
		if result != result2 {
			t.Errorf("base62Encode not deterministic: %q vs %q for input %v", result, result2, src)
		}
	})
}

func isBase62Char(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

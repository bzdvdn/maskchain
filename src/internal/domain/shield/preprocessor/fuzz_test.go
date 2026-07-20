package preprocessor

import (
	"testing"
)

func FuzzCountCommasOutsideQuotes(f *testing.F) {
	seeds := []string{
		"",
		",",
		"a,b,c",
		`"a,b",c`,
		`"a""b",c`,
		`"a,b","c,d"`,
		"line without comma",
		`"unescaped"`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		count := countCommasOutsideQuotes(s)
		if count < 0 {
			t.Errorf("countCommasOutsideQuotes(%q) = %d, expected >= 0", s, count)
		}
		if count > len(s) {
			t.Errorf("countCommasOutsideQuotes(%q) = %d > len=%d", s, count, len(s))
		}
	})
}

func FuzzParseJSONPath(f *testing.F) {
	seeds := []string{
		"",
		"items",
		"items[*]",
		"items[0].name",
		"items[0]",
		`items.*`,
		"a.b.c",
		"[*]",
		"[0]",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, path string) {
		segments, err := parseJSONPath(path)
		if err != nil {
			// Expected error — just verify no panic
			return
		}
		if len(segments) == 0 {
			t.Errorf("parseJSONPath(%q) returned nil segments without error", path)
		}
		for i, seg := range segments {
			if seg.key == "" {
				t.Errorf("segment %d in path %q has empty key", i, path)
			}
		}
	})
}

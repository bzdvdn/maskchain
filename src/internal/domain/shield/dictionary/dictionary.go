package dictionary

import "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"

// @sk-task 24-shield-dictionaries#T1.1: Implement Dictionary value object (AC-001)
type Dictionary struct {
	profileSlug value.ProfileSlug
	name        string
	entries     []interface{}
	matchMode   MatchMode
}

func NewDictionary(profileSlug value.ProfileSlug, name string, entries []interface{}, matchMode MatchMode) *Dictionary {
	return &Dictionary{
		profileSlug: profileSlug,
		name:        name,
		entries:     entries,
		matchMode:   matchMode,
	}
}

func (d *Dictionary) ProfileSlug() value.ProfileSlug { return d.profileSlug }
func (d *Dictionary) Name() string                   { return d.name }
func (d *Dictionary) Entries() []interface{}          { return d.entries }
func (d *Dictionary) MatchMode() MatchMode           { return d.matchMode }

// AllValues recursively extracts all unique string values from structured entries.
// Flattens {"name":"John","email":"j@c"} into ["John","j@c"].
func (d *Dictionary) AllValues() []string {
	seen := make(map[string]struct{})
	var result []string
	for _, entry := range d.entries {
		result = append(result, extractValues(entry, seen)...)
	}
	return result
}

func extractValues(v interface{}, seen map[string]struct{}) []string {
	switch val := v.(type) {
	case string:
		if _, ok := seen[val]; !ok {
			seen[val] = struct{}{}
			return []string{val}
		}
		return nil
	case map[string]interface{}:
		var result []string
		for _, fv := range val {
			result = append(result, extractValues(fv, seen)...)
		}
		return result
	case []interface{}:
		var result []string
		for _, item := range val {
			result = append(result, extractValues(item, seen)...)
		}
		return result
	default:
		return nil
	}
}

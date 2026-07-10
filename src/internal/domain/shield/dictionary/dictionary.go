package dictionary

import "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"

// @sk-task 24-shield-dictionaries#T1.1: Implement Dictionary value object (AC-001)
type Dictionary struct {
	profileSlug value.ProfileSlug
	name        string
	entries     []string
	matchMode   MatchMode
}

func NewDictionary(profileSlug value.ProfileSlug, name string, entries []string, matchMode MatchMode) *Dictionary {
	return &Dictionary{
		profileSlug: profileSlug,
		name:        name,
		entries:     entries,
		matchMode:   matchMode,
	}
}

func (d *Dictionary) ProfileSlug() value.ProfileSlug { return d.profileSlug }
func (d *Dictionary) Name() string                   { return d.name }
func (d *Dictionary) Entries() []string              { return d.entries }
func (d *Dictionary) MatchMode() MatchMode           { return d.matchMode }

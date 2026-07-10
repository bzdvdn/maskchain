package value

import (
	"fmt"
	"regexp"
)

var validSlug = regexp.MustCompile(`^[a-zA-Z0-9-]{3,}$`)

// @sk-task 20-shield-domain#T1.1: Implement ProfileSlug value object with validation (AC-002, AC-006)
type ProfileSlug struct {
	value string
}

func NewProfileSlug(v string) (ProfileSlug, error) {
	if !validSlug.MatchString(v) {
		return ProfileSlug{}, fmt.Errorf("invalid slug %q: must match %s", v, validSlug.String())
	}
	return ProfileSlug{value: v}, nil
}

func (s ProfileSlug) String() string { return s.value }

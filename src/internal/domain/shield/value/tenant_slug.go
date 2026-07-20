package value

import (
	"fmt"
	"regexp"
)

var validSlug = regexp.MustCompile(`^[a-zA-Z0-9-]{3,}$`)

// @sk-task tenant-profile-sync#T1.1: Implement TenantSlug VO (AC-001, AC-002)
//
// TenantSlug represents a domain entity or configuration.
type TenantSlug struct {
	value string
}

func NewTenantSlug(v string) (TenantSlug, error) {
	if !validSlug.MatchString(v) {
		return TenantSlug{}, fmt.Errorf("invalid tenant slug %q: must match %s", v, validSlug.String())
	}
	return TenantSlug{value: v}, nil
}

func (s TenantSlug) String() string { return s.value }

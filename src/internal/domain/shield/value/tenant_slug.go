package value

import (
	"fmt"
)

// @sk-task tenant-profile-sync#T1.1: Implement TenantSlug VO (AC-001, AC-002)
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

package value

import "fmt"

// @sk-task 20-shield-domain#T1.1: Implement TenantID value object (AC-006)
//
// TenantID represents a domain entity or configuration.
type TenantID struct {
	value string
}

func NewTenantID(v string) (TenantID, error) {
	if v == "" {
		return TenantID{}, fmt.Errorf("tenant id must not be empty")
	}
	return TenantID{value: v}, nil
}

func (id TenantID) String() string { return id.value }

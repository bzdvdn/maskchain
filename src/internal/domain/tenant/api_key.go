package tenant

import "fmt"

// @sk-task 80-tenant-isolation#T1.1: APIKey value object (AC-001, AC-003)
//
// APIKey represents a domain entity or configuration.
type APIKey struct {
	value string
}

func NewAPIKey(v string) (APIKey, error) {
	if v == "" {
		return APIKey{}, fmt.Errorf("api key must not be empty")
	}
	return APIKey{value: v}, nil
}

func (k APIKey) String() string { return k.value }

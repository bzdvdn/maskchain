package value

import "fmt"

// @sk-task 20-shield-domain#T1.1: Implement PatternID value object (AC-006)
//
// PatternID represents a domain entity or configuration.
type PatternID struct {
	value string
}

func NewPatternID(v string) (PatternID, error) {
	if v == "" {
		return PatternID{}, fmt.Errorf("pattern id must not be empty")
	}
	return PatternID{value: v}, nil
}

func (id PatternID) String() string { return id.value }

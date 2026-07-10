package value

import "fmt"

// @sk-task 20-shield-domain#T1.1: Implement ProfileID value object (AC-006)
type ProfileID struct {
	value string
}

func NewProfileID(v string) (ProfileID, error) {
	if v == "" {
		return ProfileID{}, fmt.Errorf("profile id must not be empty")
	}
	return ProfileID{value: v}, nil
}

func (id ProfileID) String() string { return id.value }

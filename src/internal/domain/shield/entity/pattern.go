package entity

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.2: Implement Pattern entity (AC-008)
//
// Pattern represents a domain entity or configuration.
type Pattern struct {
	id          value.PatternID
	expression  string
	description string
}

func NewPattern(id value.PatternID, expression string, description string) (*Pattern, error) {
	if expression == "" {
		return nil, errors.ErrInvalidPattern
	}
	return &Pattern{
		id:          id,
		expression:  expression,
		description: description,
	}, nil
}

func (p *Pattern) ID() value.PatternID { return p.id }
func (p *Pattern) Expression() string  { return p.expression }
func (p *Pattern) Description() string { return p.description }

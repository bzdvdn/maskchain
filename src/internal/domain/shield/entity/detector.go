package entity

import (
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.2: Implement Detector entity (AC-008)
type Detector struct {
	id       string
	typ      DetectorType
	patterns []Pattern
	severity value.Severity
	enabled  bool
}

type DetectorOption func(*Detector)

func WithDetectorEnabled(enabled bool) DetectorOption {
	return func(d *Detector) { d.enabled = enabled }
}

func NewDetector(id string, typ DetectorType, patterns []Pattern, severity value.Severity, opts ...DetectorOption) (*Detector, error) {
	if len(patterns) == 0 {
		return nil, errors.ErrInvalidPattern
	}
	d := &Detector{
		id:       id,
		typ:      typ,
		patterns: patterns,
		severity: severity,
		enabled:  true,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

func (d *Detector) ID() string               { return d.id }
func (d *Detector) Type() DetectorType       { return d.typ }
func (d *Detector) Patterns() []Pattern      { return d.patterns }
func (d *Detector) Severity() value.Severity { return d.severity }
func (d *Detector) Enabled() bool            { return d.enabled }

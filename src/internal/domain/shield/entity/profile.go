package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.1: Implement Profile entity (AC-001, AC-002)
type Profile struct {
	id          value.ProfileID
	slug        value.ProfileSlug
	tenantID    value.TenantID
	name        string
	description *string
	detectors   []Detector
	dictionaries []*dictionary.Dictionary
	enabled     bool
	createdAt   time.Time
	updatedAt   time.Time
}

type ProfileOption func(*Profile)

func WithDescription(d string) ProfileOption {
	return func(p *Profile) { p.description = &d }
}

func WithDetectors(detectors []Detector) ProfileOption {
	return func(p *Profile) { p.detectors = detectors }
}

func WithEnabled(enabled bool) ProfileOption {
	return func(p *Profile) { p.enabled = enabled }
}

// @sk-task 24-shield-dictionaries#T2.2: Add WithDictionaries option (AC-006)
func WithDictionaries(dicts []*dictionary.Dictionary) ProfileOption {
	return func(p *Profile) { p.dictionaries = dicts }
}

func NewProfile(id value.ProfileID, slug value.ProfileSlug, tenantID value.TenantID, name string, opts ...ProfileOption) *Profile {
	p := &Profile{
		id:        id,
		slug:      slug,
		tenantID:  tenantID,
		name:      name,
		detectors: nil,
		enabled:   true,
		createdAt: time.Now().UTC(),
		updatedAt: time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Profile) ID() value.ProfileID           { return p.id }
func (p *Profile) Slug() value.ProfileSlug        { return p.slug }
func (p *Profile) TenantID() value.TenantID       { return p.tenantID }
func (p *Profile) Name() string                   { return p.name }
func (p *Profile) Description() *string           { return p.description }
func (p *Profile) Detectors() []Detector          { return p.detectors }
func (p *Profile) Enabled() bool                  { return p.enabled }
func (p *Profile) Dictionaries() []*dictionary.Dictionary { return p.dictionaries }
func (p *Profile) CreatedAt() time.Time                  { return p.createdAt }
func (p *Profile) UpdatedAt() time.Time                  { return p.updatedAt }

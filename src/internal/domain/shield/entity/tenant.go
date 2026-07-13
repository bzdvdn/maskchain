package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task tenant-profile-sync#T1.1: Implement Tenant entity (AC-001, AC-002)
type Tenant struct {
	slug         value.TenantSlug
	name         string
	authHeader   string
	apiKeys      []string
	dictionaries []*dictionary.Dictionary
	createdAt    time.Time
	updatedAt    time.Time
}

type TenantOption func(*Tenant)

func WithTenantDictionaries(dicts []*dictionary.Dictionary) TenantOption {
	return func(t *Tenant) { t.dictionaries = dicts }
}

func NewTenant(slug value.TenantSlug, name string, authHeader string, apiKeys []string, opts ...TenantOption) *Tenant {
	t := &Tenant{
		slug:       slug,
		name:       name,
		authHeader: authHeader,
		apiKeys:    apiKeys,
		createdAt:  time.Now().UTC(),
		updatedAt:  time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Tenant) Slug() value.TenantSlug                    { return t.slug }
func (t *Tenant) Name() string                              { return t.name }
func (t *Tenant) AuthHeader() string                        { return t.authHeader }
func (t *Tenant) APIKeys() []string                         { return t.apiKeys }
func (t *Tenant) Dictionaries() []*dictionary.Dictionary    { return t.dictionaries }
func (t *Tenant) CreatedAt() time.Time                      { return t.createdAt }
func (t *Tenant) UpdatedAt() time.Time                      { return t.updatedAt }

package compliance

import (
	"context"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// ErrPackNotFound is returned when the requested pack key is unknown.
var ErrPackNotFound = fmt.Errorf("compliance pack not found")

// ErrPackInvalid is returned when a preset cannot be applied to a tenant.
var ErrPackInvalid = fmt.Errorf("compliance pack is invalid")

// TenantRule is a single expected rule derived from a pack preset.
type TenantRule struct {
	DetectorType entity.DetectorType `json:"detector_type"`
	Reaction     entity.Reaction     `json:"reaction"`
	Masking      bool                `json:"masking"`
}

// ApplyResult is the outcome of applying a pack to a tenant.
type ApplyResult struct {
	PackKey string       `json:"pack_key"`
	Rules   []TenantRule `json:"rules"`
}

// ApplyPackService applies a compliance pack to a tenant in one atomic action.
type ApplyPackService struct {
	registry *compliance.Registry
	repo     shield.TenantRepository
}

// NewApplyPackService creates an apply service over the given pack registry
// and tenant repository.
func NewApplyPackService(registry *compliance.Registry, repo shield.TenantRepository) *ApplyPackService {
	return &ApplyPackService{registry: registry, repo: repo}
}

// Apply resolves the pack and writes the tenant's shield config from the
// preset in a single repository Update. On any error (unknown pack, invalid
// preset, missing tenant) the tenant is left unchanged.
func (s *ApplyPackService) Apply(ctx context.Context, slug value.TenantSlug, packKey string) (*ApplyResult, error) {
	if s.registry == nil {
		return nil, fmt.Errorf("%w: registry not configured", ErrPackInvalid)
	}
	pack := s.registry.Pack(packKey)
	if pack == nil {
		return nil, fmt.Errorf("%w: %s", ErrPackNotFound, packKey)
	}
	if err := validatePackForApply(pack); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrPackInvalid, packKey, err)
	}

	tenant, err := s.repo.Get(ctx, slug)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, shielderrors.ErrTenantNotFound
	}

	rules := rulesFromPack(pack)
	pii := entity.PIIConfig{
		Enabled:       true,
		DefaultAction: string(entity.ReactionBlock),
		Rules:         toPIARules(rules),
	}
	updated := entity.NewTenant(
		tenant.Slug(),
		tenant.Name(),
		tenant.AuthHeader(),
		entity.WithTenantDictionaries(tenant.Dictionaries()),
		entity.WithTenantPIIConfig(pii),
	)

	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, err
	}
	return &ApplyResult{PackKey: packKey, Rules: rules}, nil
}

// validatePackForApply is a structural guard so an empty or malformed preset
// is rejected before any tenant write.
func validatePackForApply(pack *compliance.Pack) error {
	if pack.Key == "" {
		return fmt.Errorf("pack key is empty")
	}
	if len(pack.Rules) == 0 {
		return fmt.Errorf("pack %q has no rules", pack.Key)
	}
	return nil
}

// rulesFromPack flattens a pack's rules into TenantRule expectations.
func rulesFromPack(pack *compliance.Pack) []TenantRule {
	out := make([]TenantRule, 0, len(pack.Rules))
	for _, r := range pack.Rules {
		out = append(out, TenantRule{
			DetectorType: r.DetectorType,
			Reaction:     r.Reaction,
			Masking:      r.Masking,
		})
	}
	return out
}

// toPIARules maps pack-derived rules into the tenant PII config shape used by
// the shield middleware.
func toPIARules(rules []TenantRule) []entity.PIARule {
	out := make([]entity.PIARule, 0, len(rules))
	for _, r := range rules {
		out = append(out, entity.PIARule{
			Label:  string(r.DetectorType),
			Type:   string(r.DetectorType),
			Action: string(r.Reaction),
		})
	}
	return out
}

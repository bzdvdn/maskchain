package compliance

import (
	"context"
	"fmt"
	"sort"

	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	shielderrors "github.com/bzdvdn/maskchain/src/internal/domain/shield/errors"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// RuleState is the compliance status of a single pack rule on a tenant.
type RuleState string

const (
	// RuleStateActive means the tenant config matches the preset.
	RuleStateActive RuleState = "active"
	// RuleStateDeviated means the tenant config differs from the preset.
	RuleStateDeviated RuleState = "deviated"
	// RuleStateMissing means the preset expects the rule but it is absent.
	RuleStateMissing RuleState = "missing"
)

// ReportRule is a single row in a compliance report.
type ReportRule struct {
	DetectorType entity.DetectorType `json:"detector_type"`
	Expected     entity.Reaction     `json:"expected_reaction"`
	Actual       *entity.Reaction    `json:"actual_reaction,omitempty"`
	Masking      bool                `json:"masking"`
	Status       RuleState           `json:"status"`
}

// ComplianceReport is the per-pack report for a tenant: active rules plus any
// deviations (customized/disabled/absent rules).
type ComplianceReport struct {
	PackKey string       `json:"pack_key"`
	Rules   []ReportRule `json:"rules"`
}

// Active returns the rules matching the preset.
func (r *ComplianceReport) Active() []ReportRule {
	var out []ReportRule
	for _, rr := range r.Rules {
		if rr.Status == RuleStateActive {
			out = append(out, rr)
		}
	}
	return out
}

// Deviated returns the rules that diverge from the preset (customized or
// missing), preserving the preset's expectations.
func (r *ComplianceReport) Deviated() []ReportRule {
	var out []ReportRule
	for _, rr := range r.Rules {
		if rr.Status != RuleStateActive {
			out = append(out, rr)
		}
	}
	return out
}

// ComplianceReportService computes a per-pack compliance report for a tenant.
type ComplianceReportService struct {
	registry *compliance.Registry
	repo     shield.TenantRepository
}

// NewComplianceReportService creates a report service over the given pack
// registry and tenant repository.
func NewComplianceReportService(registry *compliance.Registry, repo shield.TenantRepository) *ComplianceReportService {
	return &ComplianceReportService{registry: registry, repo: repo}
}

// Report diffs the tenant's current shield config against the pack preset.
// Customizations are flagged as deviated and never overwritten.
func (s *ComplianceReportService) Report(ctx context.Context, slug value.TenantSlug, packKey string) (*ComplianceReport, error) {
	if s.registry == nil {
		return nil, fmt.Errorf("%w: registry not configured", ErrPackInvalid)
	}
	pack := s.registry.Pack(packKey)
	if pack == nil {
		return nil, fmt.Errorf("%w: %s", ErrPackNotFound, packKey)
	}
	tenant, err := s.repo.Get(ctx, slug)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, shielderrors.ErrTenantNotFound
	}

	pii := tenant.PIIConfig()
	actual := make(map[entity.DetectorType]entity.Reaction, len(pii.Rules))
	for _, r := range pii.Rules {
		dt := entity.DetectorType(r.Type)
		if dt == "" {
			continue
		}
		if action := entity.Reaction(r.Action); action != "" {
			actual[dt] = action
		}
	}

	rules := make([]ReportRule, 0, len(pack.Rules))
	for _, pr := range pack.Rules {
		rr := ReportRule{
			DetectorType: pr.DetectorType,
			Expected:     pr.Reaction,
			Masking:      pr.Masking,
		}
		act, ok := actual[pr.DetectorType]
		switch {
		case !ok:
			rr.Status = RuleStateMissing
		case act != pr.Reaction:
			rr.Status = RuleStateDeviated
			rr.Actual = &act
		default:
			rr.Status = RuleStateActive
		}
		rules = append(rules, rr)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].DetectorType < rules[j].DetectorType })

	return &ComplianceReport{PackKey: packKey, Rules: rules}, nil
}

package compliance

import (
	"fmt"
	"sort"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// PackRule describes one shield element a compliance pack expects to be active
// on a tenant. DetectorType and Reaction reference existing shield IDs by name.
type PackRule struct {
	DetectorType entity.DetectorType `yaml:"detector_type" json:"detector_type"`
	Reaction     entity.Reaction     `yaml:"reaction" json:"reaction"`
	Masking      bool                `yaml:"masking" json:"masking"`
}

// Pack is a compliance preset: a keyed set of shield rule expectations.
// It groups existing detectors/reactions/masking — it never redefines them.
type Pack struct {
	Key   string     `yaml:"key" json:"key"`
	Name  string     `yaml:"name" json:"name"`
	Rules []PackRule `yaml:"rules" json:"rules"`
}

// Catalog is the set of shield element identifiers a pack may reference.
// It is built from the live shield registry so unknown IDs are caught at load.
type Catalog struct {
	detectorTypes map[entity.DetectorType]struct{}
	reactions     map[entity.Reaction]struct{}
}

// NewCatalogFromRegistry derives the allowed detector/reaction IDs from the
// shield registry (DetectorRegistry.Types) and the reaction constants.
func NewCatalogFromRegistry(reg *detector.DetectorRegistry) Catalog {
	c := Catalog{
		detectorTypes: make(map[entity.DetectorType]struct{}),
		reactions:     make(map[entity.Reaction]struct{}),
	}
	if reg != nil {
		for _, t := range reg.Types() {
			c.detectorTypes[t] = struct{}{}
		}
	}
	for _, r := range []entity.Reaction{entity.ReactionAllow, entity.ReactionBlock, entity.ReactionReview, entity.ReactionLog} {
		c.reactions[r] = struct{}{}
	}
	return c
}

// NewCatalog allows constructing a Catalog from explicit sets (used in tests
// and when the registry is not yet populated).
func NewCatalog(types []entity.DetectorType, reactions []entity.Reaction) Catalog {
	c := Catalog{
		detectorTypes: make(map[entity.DetectorType]struct{}),
		reactions:     make(map[entity.Reaction]struct{}),
	}
	for _, t := range types {
		c.detectorTypes[t] = struct{}{}
	}
	for _, r := range reactions {
		c.reactions[r] = struct{}{}
	}
	return c
}

// Validate checks the pack against the catalog: every referenced detector and
// reaction must be known. Returns a per-pack error on the first unknown ID.
func (p *Pack) Validate(cat Catalog) error {
	if p.Key == "" {
		return fmt.Errorf("pack key is required")
	}
	if len(p.Rules) == 0 {
		return fmt.Errorf("pack %q: no rules", p.Key)
	}
	for i, r := range p.Rules {
		if _, ok := cat.detectorTypes[r.DetectorType]; !ok {
			return fmt.Errorf("pack %q: rule %d references unknown detector type %q", p.Key, i, r.DetectorType)
		}
		if _, ok := cat.reactions[r.Reaction]; !ok {
			return fmt.Errorf("pack %q: rule %d references unknown reaction %q", p.Key, i, r.Reaction)
		}
	}
	return nil
}

// RuleFor returns the rule matching the given detector type, or nil.
func (p *Pack) RuleFor(typ entity.DetectorType) *PackRule {
	for i := range p.Rules {
		if p.Rules[i].DetectorType == typ {
			return &p.Rules[i]
		}
	}
	return nil
}

// DetectorTypes returns the pack's detector types in stable order.
func (p *Pack) DetectorTypes() []entity.DetectorType {
	out := make([]entity.DetectorType, 0, len(p.Rules))
	for _, r := range p.Rules {
		out = append(out, r.DetectorType)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

package profilerepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/dictionary"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 102-profile-cache#T2.1: Implement ProfileValkeyRepo (RQ-002, RQ-003, RQ-009)
type ProfileValkeyRepo struct {
	client valkey.Client
	ttl    time.Duration
}

func NewProfileValkeyRepo(client valkey.Client, ttl time.Duration) *ProfileValkeyRepo {
	return &ProfileValkeyRepo{client: client, ttl: ttl}
}

func (r *ProfileValkeyRepo) key(tenantID, slug string) string {
	return "profile:" + tenantID + ":" + slug
}

func (r *ProfileValkeyRepo) Get(ctx context.Context, tenantID, slug string) (*profileCacheValue, error) {
	if r.client == nil {
		return nil, nil
	}
	data, err := r.client.Do(ctx, r.client.B().Get().Key(r.key(tenantID, slug)).Build()).ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, err
	}
	var val profileCacheValue
	if err := json.Unmarshal([]byte(data), &val); err != nil {
		return nil, fmt.Errorf("unmarshal profile cache value: %w", err)
	}
	return &val, nil
}

func (r *ProfileValkeyRepo) Set(ctx context.Context, tenantID, slug string, val *profileCacheValue) error {
	if r.client == nil {
		return nil
	}
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal profile cache value: %w", err)
	}
	return r.client.Do(ctx, r.client.B().Set().Key(r.key(tenantID, slug)).
		Value(string(data)).
		Ex(r.ttl).
		Build()).Error()
}

func (r *ProfileValkeyRepo) Del(ctx context.Context, tenantID, slug string) error {
	if r.client == nil {
		return nil
	}
	return r.client.Do(ctx, r.client.B().Del().Key(r.key(tenantID, slug)).Build()).Error()
}

// @sk-task 102-profile-cache#T3.1: Add PubSub publish to ProfileCache (AC-005, AC-007)
func (r *ProfileValkeyRepo) Publish(ctx context.Context, slug string) error {
	if r.client == nil {
		return nil
	}
	return r.client.Do(ctx, r.client.B().Publish().Channel("profile.invalidate:"+slug).Message("").Build()).Error()
}

// profileCacheValue is a JSON-serializable DTO for full profile storage in Valkey.
// It mirrors entity.Profile fields (which are unexported in the domain entity).
type profileCacheValue struct {
	ID            string                          `json:"id"`
	Slug          string                          `json:"slug"`
	TenantID      string                          `json:"tenant_id"`
	Name          string                          `json:"name"`
	Description   *string                         `json:"description,omitempty"`
	Detectors     []detectorDTO                   `json:"detectors"`
	Dictionaries  []dictionaryDTO                 `json:"dictionaries"`
	Preprocessors []preprocessorDTO               `json:"preprocessors"`
	Enabled       bool                            `json:"enabled"`
	Version       int                             `json:"version"`
	CreatedAt     time.Time                       `json:"created_at"`
	UpdatedAt     time.Time                       `json:"updated_at"`
}

type detectorDTO struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Patterns []patternDTO `json:"patterns"`
	Severity string       `json:"severity"`
	Enabled  bool         `json:"enabled"`
}

type patternDTO struct {
	ID          string `json:"id"`
	Expression  string `json:"expression"`
	Description string `json:"description"`
}

type dictionaryDTO struct {
	ProfileSlug string   `json:"profile_slug"`
	Name        string   `json:"name"`
	Entries     []string `json:"entries"`
	MatchMode   string   `json:"match_mode"`
}

type preprocessorDTO = preprocessor.PreprocessorDef

func profileToCacheValue(p *entity.Profile, version int) *profileCacheValue {
	detDTOs := make([]detectorDTO, len(p.Detectors()))
	for i, d := range p.Detectors() {
		patDTOs := make([]patternDTO, len(d.Patterns()))
		for j, pat := range d.Patterns() {
			patDTOs[j] = patternDTO{
				ID:          pat.ID().String(),
				Expression:  pat.Expression(),
				Description: pat.Description(),
			}
		}
		detDTOs[i] = detectorDTO{
			ID:       d.ID(),
			Type:     string(d.Type()),
			Patterns: patDTOs,
			Severity: d.Severity().String(),
			Enabled:  d.Enabled(),
		}
	}

	dictDTOs := make([]dictionaryDTO, len(p.Dictionaries()))
	for i, d := range p.Dictionaries() {
		dictDTOs[i] = dictionaryDTO{
			ProfileSlug: d.ProfileSlug().String(),
			Name:        d.Name(),
			Entries:     d.Entries(),
			MatchMode:   string(d.MatchMode()),
		}
	}

	var desc *string
	if d := p.Description(); d != nil {
		desc = d
	}

	return &profileCacheValue{
		ID:            p.ID().String(),
		Slug:          p.Slug().String(),
		TenantID:      p.TenantID().String(),
		Name:          p.Name(),
		Description:   desc,
		Detectors:     detDTOs,
		Dictionaries:  dictDTOs,
		Preprocessors: p.Preprocessors(),
		Enabled:       p.Enabled(),
		Version:       version,
		CreatedAt:     p.CreatedAt(),
		UpdatedAt:     p.UpdatedAt(),
	}
}

func cacheValueToProfile(v *profileCacheValue) (*entity.Profile, error) {
	pid, err := value.NewProfileID(v.ID)
	if err != nil {
		return nil, fmt.Errorf("parse profile id: %w", err)
	}
	slug, err := value.NewProfileSlug(v.Slug)
	if err != nil {
		return nil, fmt.Errorf("parse slug: %w", err)
	}
	tid, err := value.NewTenantID(v.TenantID)
	if err != nil {
		return nil, fmt.Errorf("parse tenant id: %w", err)
	}

	var opts []entity.ProfileOption
	opts = append(opts, entity.WithEnabled(v.Enabled))

	if v.Description != nil {
		opts = append(opts, entity.WithDescription(*v.Description))
	}

	dets := make([]entity.Detector, 0, len(v.Detectors))
	for _, d := range v.Detectors {
		patterns := make([]entity.Pattern, 0, len(d.Patterns))
		for _, pat := range d.Patterns {
			patID, err := value.NewPatternID(pat.ID)
			if err != nil {
				return nil, fmt.Errorf("parse pattern id: %w", err)
			}
			p, err := entity.NewPattern(patID, pat.Expression, pat.Description)
			if err != nil {
				return nil, fmt.Errorf("reconstruct pattern: %w", err)
			}
			patterns = append(patterns, *p)
		}

		sev := severityFromString(d.Severity)
		det, err := entity.NewDetector(d.ID, entity.DetectorType(d.Type), patterns, sev, entity.WithDetectorEnabled(d.Enabled))
		if err != nil {
			return nil, fmt.Errorf("reconstruct detector: %w", err)
		}
		dets = append(dets, *det)
	}

	dicts := make([]*dictionary.Dictionary, 0, len(v.Dictionaries))
	for _, d := range v.Dictionaries {
		ds, err := value.NewProfileSlug(d.ProfileSlug)
		if err != nil {
			return nil, fmt.Errorf("parse dict profile slug: %w", err)
		}
		dicts = append(dicts, dictionary.NewDictionary(ds, d.Name, d.Entries, dictionary.MatchMode(d.MatchMode)))
	}

	if len(dets) > 0 {
		opts = append(opts, entity.WithDetectors(dets))
	}
	if len(dicts) > 0 {
		opts = append(opts, entity.WithDictionaries(dicts))
	}
	if len(v.Preprocessors) > 0 {
		opts = append(opts, entity.WithPreprocessors(v.Preprocessors))
	}

	return entity.NewProfile(pid, slug, tid, v.Name, opts...), nil
}

func severityFromString(s string) value.Severity {
	switch s {
	case "low":
		return value.SeverityLow
	case "medium":
		return value.SeverityMedium
	case "high":
		return value.SeverityHigh
	case "critical":
		return value.SeverityCritical
	default:
		return value.SeverityLow
	}
}

func metadataKey(tenantID, slug string) string {
	return tenantID + ":" + slug
}

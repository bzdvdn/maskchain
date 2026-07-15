package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.3: Implement Incident entity
// @sk-task 30-shield-persistence#T3.1: Add persistence fields to Incident (DM-003)
// @sk-task 60-audit-incidents#T1.2: Add tenant, responseSnippet, rename rawSnippet -> promptSnippetRedacted (AC-001, AC-002)
type Incident struct {
	detectorID string
	patternID  value.PatternID
	severity   value.Severity
	fragment   string
	position   int

	slug         string
	requestID    string
	detectorType string
	entryValue   *string
	action       string
	promptSnippetRedacted *string
	responseSnippet       *string
	tenant                string
	timestamp   time.Time
}

func NewIncident(detectorID string, patternID value.PatternID, severity value.Severity, fragment string, position int) *Incident {
	return &Incident{
		detectorID: detectorID,
		patternID:  patternID,
		severity:   severity,
		fragment:   fragment,
		position:   position,
	}
}

// @sk-task cleanup-profile-repository#T3.1: Remove profileSlug from NewAuditIncident (AC-012)
// @sk-task 60-audit-incidents#T1.2: Add tenant, responseSnippet, promptSnippetRedacted params (AC-001, AC-002)
func NewAuditIncident(slug, requestID, detectorType string, entryValue *string, severity value.Severity, action string, promptSnippetRedacted, responseSnippet *string, tenant string, timestamp time.Time) *Incident {
	return &Incident{
		slug:                  slug,
		requestID:             requestID,
		detectorType:          detectorType,
		entryValue:            entryValue,
		severity:              severity,
		action:                action,
		promptSnippetRedacted: promptSnippetRedacted,
		responseSnippet:       responseSnippet,
		tenant:                tenant,
		timestamp:             timestamp,
	}
}

func (inc *Incident) DetectorID() string           { return inc.detectorID }
func (inc *Incident) PatternID() value.PatternID   { return inc.patternID }
func (inc *Incident) Severity() value.Severity     { return inc.severity }
func (inc *Incident) Fragment() string             { return inc.fragment }
func (inc *Incident) Position() int                { return inc.position }
func (inc *Incident) Slug() string                 { return inc.slug }
func (inc *Incident) RequestID() string            { return inc.requestID }
func (inc *Incident) DetectorType() string         { return inc.detectorType }
func (inc *Incident) EntryValue() *string          { return inc.entryValue }
func (inc *Incident) Action() string               { return inc.action }
func (inc *Incident) RawSnippet() *string          { return inc.promptSnippetRedacted }

// @sk-task 60-audit-incidents#T1.2: Getter for renamed field (AC-001, AC-002)
func (inc *Incident) PromptSnippetRedacted() *string { return inc.promptSnippetRedacted }

// @sk-task 60-audit-incidents#T1.2: Getter for new field (AC-001, AC-002)
func (inc *Incident) ResponseSnippet() *string { return inc.responseSnippet }

// @sk-task 60-audit-incidents#T1.2: Getter for new field (AC-001, AC-002)
func (inc *Incident) Tenant() string { return inc.tenant }
func (inc *Incident) Timestamp() time.Time         { return inc.timestamp }

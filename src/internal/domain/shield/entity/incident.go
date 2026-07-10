package entity

import (
	"time"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 20-shield-domain#T2.3: Implement Incident entity
// @sk-task 30-shield-persistence#T3.1: Add persistence fields to Incident (DM-003)
type Incident struct {
	detectorID string
	patternID  value.PatternID
	severity   value.Severity
	fragment   string
	position   int

	slug        string
	profileSlug string
	requestID   string
	detectorType string
	entryValue  *string
	action      string
	rawSnippet  *string
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

// @sk-task 30-shield-persistence#T3.1: Add full constructor for persisted incidents (DM-003)
func NewAuditIncident(slug, profileSlug, requestID, detectorType string, entryValue *string, severity value.Severity, action string, rawSnippet *string, timestamp time.Time) *Incident {
	return &Incident{
		slug:         slug,
		profileSlug:  profileSlug,
		requestID:    requestID,
		detectorType: detectorType,
		entryValue:   entryValue,
		severity:     severity,
		action:       action,
		rawSnippet:   rawSnippet,
		timestamp:    timestamp,
	}
}

func (inc *Incident) DetectorID() string           { return inc.detectorID }
func (inc *Incident) PatternID() value.PatternID   { return inc.patternID }
func (inc *Incident) Severity() value.Severity     { return inc.severity }
func (inc *Incident) Fragment() string             { return inc.fragment }
func (inc *Incident) Position() int                { return inc.position }
func (inc *Incident) Slug() string                 { return inc.slug }
func (inc *Incident) ProfileSlug() string          { return inc.profileSlug }
func (inc *Incident) RequestID() string            { return inc.requestID }
func (inc *Incident) DetectorType() string         { return inc.detectorType }
func (inc *Incident) EntryValue() *string          { return inc.entryValue }
func (inc *Incident) Action() string               { return inc.action }
func (inc *Incident) RawSnippet() *string          { return inc.rawSnippet }
func (inc *Incident) Timestamp() time.Time         { return inc.timestamp }

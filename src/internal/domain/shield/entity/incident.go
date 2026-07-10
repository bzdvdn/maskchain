package entity

import "github.com/bzdvdn/maskchain/src/internal/domain/shield/value"

// @sk-task 20-shield-domain#T2.3: Implement Incident entity
type Incident struct {
	detectorID string
	patternID  value.PatternID
	severity   value.Severity
	fragment   string
	position   int
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

func (inc *Incident) DetectorID() string          { return inc.detectorID }
func (inc *Incident) PatternID() value.PatternID  { return inc.patternID }
func (inc *Incident) Severity() value.Severity    { return inc.severity }
func (inc *Incident) Fragment() string            { return inc.fragment }
func (inc *Incident) Position() int               { return inc.position }

package preprocessor

import "fmt"

type MaskMode string

const (
	MaskModeFull    MaskMode = "full"
	MaskModeSurname MaskMode = "surname"
)

type Rule struct {
	Columns []string `json:"columns,omitempty"`
	Path    string   `json:"path,omitempty"`
	Mask    MaskMode `json:"mask"`
}

func (r Rule) Validate() error {
	switch r.Mask {
	case MaskModeFull, MaskModeSurname:
	default:
		return fmt.Errorf("unknown mask mode: %q", r.Mask)
	}
	if len(r.Columns) == 0 && r.Path == "" {
		return fmt.Errorf("rule must specify columns or path")
	}
	return nil
}

// @sk-task 25-shield-preprocessors#T1.1: Define PreprocessorDef struct (AC-007)
//
// PreprocessorDef represents a domain entity or configuration.
type PreprocessorDef struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Rules []Rule `json:"rules"`
}

// @sk-task 25-shield-preprocessors#T1.1: Define ProcessResult struct (AC-007)
//
// ProcessResult represents a domain entity or configuration.
type ProcessResult struct {
	ModifiedText string
	Replacements map[string]string
}

// @sk-task 25-shield-preprocessors#T1.1: Define Processor interface (AC-007)
//
// Processor defines the interface for domain operations.
type Processor interface {
	Name() string
	Process(data string, namespace string) *ProcessResult
}

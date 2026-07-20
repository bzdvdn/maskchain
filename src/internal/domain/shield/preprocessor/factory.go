package preprocessor

import "fmt"

// @sk-task 25-shield-preprocessors#T1.1: Implement NewPreprocessor factory (AC-007)
//
// NewPreprocessor creates a new Preprocessor.
func NewPreprocessor(def PreprocessorDef) (Processor, error) {
	if def.Name == "" {
		return nil, fmt.Errorf("preprocessor name is required")
	}
	if len(def.Rules) == 0 {
		return nil, fmt.Errorf("preprocessor %q has no rules", def.Name)
	}
	for i, r := range def.Rules {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("preprocessor %q rule %d: %w", def.Name, i, err)
		}
	}
	switch def.Type {
	case "csv":
		return &CSVProcessor{name: def.Name, rules: def.Rules}, nil
	case "json":
		return &JSONProcessor{name: def.Name, rules: def.Rules}, nil
	default:
		return nil, fmt.Errorf("unknown preprocessor type: %q", def.Type)
	}
}

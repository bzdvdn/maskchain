package shield

import (
	"context"
	"fmt"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-task 50-shield-engine#T2.1: Implement ScanPipelineFactory (AC-001, AC-005)
type DetectorBinding struct {
	Interface detector.Detector
	Label     string
	Severity  value.Severity
}

// @sk-task 50-shield-engine#T2.1: Implement ScanPipelineFactory (AC-001, AC-005)
type Pipeline struct {
	Preprocessors []preprocessor.Processor
	Detectors     []DetectorBinding
}

// @sk-task 50-shield-engine#T2.1: Implement ScanPipelineFactory (AC-001, AC-005)
type ScanPipelineFactory struct {
	registry *detector.DetectorRegistry
}

func NewScanPipelineFactory(registry *detector.DetectorRegistry) *ScanPipelineFactory {
	return &ScanPipelineFactory{registry: registry}
}

func (f *ScanPipelineFactory) Build(ctx context.Context, profile *entity.Profile) (*Pipeline, error) {
	preprocessors := make([]preprocessor.Processor, 0, len(profile.Preprocessors()))
	for _, def := range profile.Preprocessors() {
		p, err := preprocessor.NewPreprocessor(def)
		if err != nil {
			return nil, fmt.Errorf("build preprocessor %q: %w", def.Name, err)
		}
		preprocessors = append(preprocessors, p)
	}

	var detectors []DetectorBinding

	for _, dict := range profile.Dictionaries() {
		dd := detector.NewDictionaryDetector(dict)
		detectors = append(detectors, DetectorBinding{
			Interface: dd,
			Label:     "dictionary:" + dict.Name(),
			Severity:  value.SeverityMedium,
		})
	}

	for _, det := range profile.Detectors() {
		if !det.Enabled() {
			continue
		}
		concrete := f.registry.Get(det.Type())
		if concrete == nil {
			return nil, fmt.Errorf("no registered detector for type %q", det.Type())
		}
		detectors = append(detectors, DetectorBinding{
			Interface: concrete,
			Label:     det.ID(),
			Severity:  det.Severity(),
		})
	}

	return &Pipeline{
		Preprocessors: preprocessors,
		Detectors:     detectors,
	}, nil
}

// @sk-task 13-shield-middleware-wiring#T1.3: BuildFromRules creates detectors from tenant rules (AC-001)
func (f *ScanPipelineFactory) BuildFromRules(ctx context.Context, rules []entity.PIARule) (*Pipeline, error) {
	var detectors []DetectorBinding
	for _, rule := range rules {
		concrete := f.registry.Get(entity.DetectorType(rule.Type))
		if concrete == nil {
			return nil, fmt.Errorf("no registered detector for type %q", rule.Type)
		}
		detectors = append(detectors, DetectorBinding{
			Interface: concrete,
			Label:     rule.Label,
			Severity:  value.SeverityMedium,
		})
	}
	return &Pipeline{Detectors: detectors}, nil
}

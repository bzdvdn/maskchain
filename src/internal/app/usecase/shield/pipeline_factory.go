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
	Type      entity.DetectorType
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

// @sk-task cleanup-profile-repository#T3.5: Remove Build(ctx, profile), keep BuildFromRules (AC-006)
func NewScanPipelineFactory(registry *detector.DetectorRegistry) *ScanPipelineFactory {
	return &ScanPipelineFactory{registry: registry}
}

// @sk-task 13-shield-middleware-wiring#T1.3: BuildFromRules creates detectors from tenant rules (AC-001)
func (f *ScanPipelineFactory) BuildFromRules(ctx context.Context, rules []entity.PIARule) (*Pipeline, error) {
	detectors := make([]DetectorBinding, 0, len(rules))
	for _, rule := range rules {
		concrete := f.registry.Get(entity.DetectorType(rule.Type))
		if concrete == nil {
			return nil, fmt.Errorf("no registered detector for type %q", rule.Type)
		}
		detectors = append(detectors, DetectorBinding{
			Interface: concrete,
			Type:      entity.DetectorType(rule.Type),
			Label:     rule.Label,
			Severity:  value.SeverityMedium,
		})
	}
	return &Pipeline{Detectors: detectors}, nil
}

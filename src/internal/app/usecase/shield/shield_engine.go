package shield

import (
	"context"
)

// @sk-task 50-shield-engine#T2.1: Implement ShieldEngine public API (AC-001, AC-004, AC-005)
type ShieldEngine struct {
	useCase *ScanUseCase
}

func NewShieldEngine(useCase *ScanUseCase) *ShieldEngine {
	return &ShieldEngine{useCase: useCase}
}

func (e *ShieldEngine) Scan(ctx context.Context, req ScanRequest) (*ScanResponse, error) {
	return e.useCase.Scan(ctx, req)
}

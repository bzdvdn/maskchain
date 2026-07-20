package detector

import (
	"fmt"
	"sync"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

// @sk-task 21-shield-detectors#T1.2: Implement DetectorRegistry (AC-008)
//
// DetectorRegistry represents a domain entity or configuration.
type DetectorRegistry struct {
	mu  sync.RWMutex
	reg map[entity.DetectorType]Detector
}

func NewDetectorRegistry() *DetectorRegistry {
	return &DetectorRegistry{
		reg: make(map[entity.DetectorType]Detector),
	}
}

func (r *DetectorRegistry) Register(typ entity.DetectorType, d Detector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.reg[typ]; ok {
		return fmt.Errorf("detector %q already registered", typ)
	}
	r.reg[typ] = d
	return nil
}

func (r *DetectorRegistry) Get(typ entity.DetectorType) Detector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reg[typ]
}

func (r *DetectorRegistry) Types() []entity.DetectorType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]entity.DetectorType, 0, len(r.reg))
	for t := range r.reg {
		types = append(types, t)
	}
	return types
}

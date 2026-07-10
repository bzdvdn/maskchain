package detector

import (
	"context"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
)

type testDetector struct {
	typ entity.DetectorType
}

func (d *testDetector) Scan(_ context.Context, _ string) ([]DetectorResult, error) {
	return nil, nil
}

// @sk-test 21-shield-detectors#T1.2: TestRegisterAndGet known type (AC-008)
func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewDetectorRegistry()
	d := &testDetector{typ: entity.DetectorTypeRegex}

	err := r.Register(entity.DetectorTypeRegex, d)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := r.Get(entity.DetectorTypeRegex)
	if got == nil {
		t.Fatal("Get returned nil for registered type")
	}
}

// @sk-test 21-shield-detectors#T1.2: TestGet returns nil for unknown type (AC-008)
func TestRegistry_GetUnknown(t *testing.T) {
	r := NewDetectorRegistry()

	got := r.Get(entity.DetectorTypeKeyword)
	if got != nil {
		t.Fatal("Get should return nil for unregistered type")
	}
}

// @sk-test 21-shield-detectors#T1.2: TestRegisterDuplicate returns error (AC-008)
func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewDetectorRegistry()
	d := &testDetector{typ: entity.DetectorTypeRegex}

	err := r.Register(entity.DetectorTypeRegex, d)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err = r.Register(entity.DetectorTypeRegex, d)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

// @sk-test 21-shield-detectors#T1.2: TestTypes returns all registered types (AC-008)
func TestRegistry_Types(t *testing.T) {
	r := NewDetectorRegistry()
	_ = r.Register(entity.DetectorTypeRegex, &testDetector{})
	_ = r.Register(entity.DetectorTypeKeyword, &testDetector{})

	types := r.Types()
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
}

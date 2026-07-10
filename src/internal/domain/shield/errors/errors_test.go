package errors

import (
	"errors"
	"testing"
)

// @sk-test 20-shield-domain#T5.1: Test sentinel errors are distinct (AC-007)
// @sk-test 23-shield-reactions#T4.1: ErrBlockedByPolicy is distinct (AC-001)
func TestSentinelErrors(t *testing.T) {
	errs := []error{
		ErrProfileNotFound,
		ErrInvalidPattern,
		ErrInvalidSlug,
		ErrDetectorFailed,
		ErrDuplicateSlug,
		ErrBlockedByPolicy,
	}

	for i, a := range errs {
		for j, b := range errs {
			if i != j && errors.Is(a, b) {
				t.Errorf("errors %v and %v should be distinct", a, b)
			}
		}
	}
}

// @sk-test 20-shield-domain#T5.1: Test sentinel errors are identifiable via errors.Is (AC-007)
// @sk-test 23-shield-reactions#T4.1: ErrBlockedByPolicy is identifiable (AC-001)
func TestSentinelErrors_Is(t *testing.T) {
	if !errors.Is(ErrProfileNotFound, ErrProfileNotFound) {
		t.Error("ErrProfileNotFound should identify itself")
	}
	if !errors.Is(ErrInvalidPattern, ErrInvalidPattern) {
		t.Error("ErrInvalidPattern should identify itself")
	}
	if !errors.Is(ErrInvalidSlug, ErrInvalidSlug) {
		t.Error("ErrInvalidSlug should identify itself")
	}
	if !errors.Is(ErrDetectorFailed, ErrDetectorFailed) {
		t.Error("ErrDetectorFailed should identify itself")
	}
	if !errors.Is(ErrDuplicateSlug, ErrDuplicateSlug) {
		t.Error("ErrDuplicateSlug should identify itself")
	}
	if !errors.Is(ErrBlockedByPolicy, ErrBlockedByPolicy) {
		t.Error("ErrBlockedByPolicy should identify itself")
	}
}

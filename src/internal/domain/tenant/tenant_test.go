package tenant

import (
	"testing"
)

// @sk-test 80-tenant-isolation#T4.1: TestNewTenant creates tenant with defaults (AC-001)
func TestNewTenant(t *testing.T) {
	key, _ := NewAPIKey("sk-test-123")
	t1 := NewTenant("alpha", "Alpha Corp", "prof1", []APIKey{key}, "", "")
	if t1.Slug() != "alpha" {
		t.Errorf("expected alpha, got %s", t1.Slug())
	}
	if t1.AuthHeader() != "X-Mask-Authorization" {
		t.Errorf("expected X-Mask-Authorization, got %s", t1.AuthHeader())
	}
	if t1.AuthScheme() != "raw" {
		t.Errorf("expected raw, got %s", t1.AuthScheme())
	}
}

// @sk-test 80-tenant-isolation#T4.1: TestNewTenantAuthHeader accepts custom auth header (AC-004)
func TestNewTenantCustomAuth(t *testing.T) {
	key, _ := NewAPIKey("sk-test-456")
	t1 := NewTenant("beta", "Beta Inc", "prof2", []APIKey{key}, "X-Custom", "bearer")
	if t1.AuthHeader() != "X-Custom" {
		t.Errorf("expected X-Custom, got %s", t1.AuthHeader())
	}
	if t1.AuthScheme() != "bearer" {
		t.Errorf("expected bearer, got %s", t1.AuthScheme())
	}
}

// @sk-test 80-tenant-isolation#T4.1: TestAPIKey validation rejects empty (AC-001, AC-003)
func TestAPIKeyEmpty(t *testing.T) {
	_, err := NewAPIKey("")
	if err == nil {
		t.Error("expected error for empty api key")
	}
}

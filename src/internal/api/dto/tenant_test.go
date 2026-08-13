package dto

import (
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 402-zero-retention-mode#T2.3: Tenant response includes retention_mode (AC-002, AC-010)
func TestTenantToResponseRetentionMode(t *testing.T) {
	slug, _ := value.NewTenantSlug("acme")
	tenant := entity.NewTenant(slug, "Acme", "X-Auth", []string{"k"},
		entity.WithTenantRetentionMode(value.RetentionModeNone))

	resp := TenantToResponse(tenant)
	if resp.RetentionMode != "none" {
		t.Errorf("RetentionMode = %q, want %q", resp.RetentionMode, "none")
	}
}

// @sk-test 402-zero-retention-mode#T2.3: Tenant response omits unset retention mode (AC-001)
func TestTenantToResponseRetentionModeUnset(t *testing.T) {
	slug, _ := value.NewTenantSlug("acme")
	tenant := entity.NewTenant(slug, "Acme", "X-Auth", []string{"k"})

	resp := TenantToResponse(tenant)
	if resp.RetentionMode != "" {
		t.Errorf("RetentionMode = %q, want empty", resp.RetentionMode)
	}
}

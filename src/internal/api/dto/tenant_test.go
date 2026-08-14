package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/value"
)

// @sk-test 402-zero-retention-mode#T2.3: Tenant response includes retention_mode (AC-002, AC-010)
func TestTenantToResponseRetentionMode(t *testing.T) {
	slug, _ := value.NewTenantSlug("acme")
	tenant := entity.NewTenant(slug, "Acme", "X-Auth",
		entity.WithTenantRetentionMode(value.RetentionModeNone))

	resp := TenantToResponse(tenant)
	if resp.RetentionMode != "none" {
		t.Errorf("RetentionMode = %q, want %q", resp.RetentionMode, "none")
	}
}

// @sk-test 402-zero-retention-mode#T2.3: Tenant response omits unset retention mode (AC-001)
func TestTenantToResponseRetentionModeUnset(t *testing.T) {
	slug, _ := value.NewTenantSlug("acme")
	tenant := entity.NewTenant(slug, "Acme", "X-Auth")

	resp := TenantToResponse(tenant)
	if resp.RetentionMode != "" {
		t.Errorf("RetentionMode = %q, want empty", resp.RetentionMode)
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: Tenant DTOs carry no api_keys (AC-007)
func TestTenantDTOGrepNegativeAPIKeys(t *testing.T) {
	if _, ok := any(CreateTenantRequest{}).(interface{ getAPIKeys() []string }); ok {
		t.Error("CreateTenantRequest must not expose api_keys")
	}
	if _, ok := any(UpdateTenantRequest{}).(interface{ getAPIKeys() []string }); ok {
		t.Error("UpdateTenantRequest must not expose api_keys")
	}

	marshal := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		return string(b)
	}

	create := marshal(CreateTenantRequest{})
	if strings.Contains(create, "api_keys") {
		t.Errorf("CreateTenantRequest JSON must not contain api_keys, got %s", create)
	}
	update := marshal(UpdateTenantRequest{})
	if strings.Contains(update, "api_keys") {
		t.Errorf("UpdateTenantRequest JSON must not contain api_keys, got %s", update)
	}
	slug, _ := value.NewTenantSlug("acme")
	resp := marshal(TenantToResponse(entity.NewTenant(slug, "Acme", "X-Auth")))
	if strings.Contains(resp, "api_keys") {
		t.Errorf("TenantResponse JSON must not contain api_keys, got %s", resp)
	}
}

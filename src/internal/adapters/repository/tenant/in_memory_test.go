package tenantrepo

import (
	"testing"

	"github.com/bzdvdn/maskchain/src/internal/domain/tenant"
)

func NewInMemoryRepositoryForTest(tenants []*tenant.Tenant) (*InMemoryRepository, error) {
	return NewInMemoryRepository(tenants)
}

// @sk-test 80-tenant-isolation#T4.1: TestInMemoryRepoFindByAPIKey (AC-001, AC-003)
func TestInMemoryRepoFindByAPIKey(t *testing.T) {
	k1, _ := tenant.NewAPIKey("key-alpha")
	k2, _ := tenant.NewAPIKey("key-beta")
	t1 := tenant.NewTenant("alpha", "Alpha", "p1", []tenant.APIKey{k1}, "", "")
	t2 := tenant.NewTenant("beta", "Beta", "p2", []tenant.APIKey{k2}, "X-Custom", "raw")

	repo, err := NewInMemoryRepository([]*tenant.Tenant{t1, t2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := repo.FindByAPIKey("key-alpha")
	if !ok {
		t.Fatal("expected to find key-alpha")
	}
	if got.Slug() != "alpha" {
		t.Errorf("expected alpha, got %s", got.Slug())
	}

	_, ok = repo.FindByAPIKey("key-unknown")
	if ok {
		t.Error("expected not to find unknown key")
	}
}

// @sk-test 80-tenant-isolation#T4.1: TestInMemoryRepoDuplicateKeyFails (AC-001, AC-003)
func TestInMemoryRepoDuplicateKeyFails(t *testing.T) {
	k1, _ := tenant.NewAPIKey("dup-key")
	k2, _ := tenant.NewAPIKey("dup-key")
	t1 := tenant.NewTenant("alpha", "Alpha", "p1", []tenant.APIKey{k1}, "", "")
	t2 := tenant.NewTenant("beta", "Beta", "p2", []tenant.APIKey{k2}, "", "")

	_, err := NewInMemoryRepository([]*tenant.Tenant{t1, t2})
	if err == nil {
		t.Error("expected error for duplicate key")
	}
}

// @sk-test 80-tenant-isolation#T4.1: TestInMemoryRepoFindBySlug (AC-005)
func TestInMemoryRepoFindBySlug(t *testing.T) {
	k, _ := tenant.NewAPIKey("some-key")
	t1 := tenant.NewTenant("gamma", "Gamma", "p3", []tenant.APIKey{k}, "", "")

	repo, err := NewInMemoryRepository([]*tenant.Tenant{t1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := repo.FindBySlug("gamma")
	if !ok {
		t.Fatal("expected to find gamma")
	}
	if got.Name() != "Gamma" {
		t.Errorf("expected Gamma, got %s", got.Name())
	}
}

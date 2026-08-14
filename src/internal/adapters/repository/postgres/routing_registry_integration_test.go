//go:build integration

package postgres

import (
	"bytes"
	"context"
	"testing"

	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-test 403-key-at-rest-encryption#T5.1: TestPgRegistrySecretsCiphertextAtRest (AC-001, AC-002)
func TestPgRegistrySecretsCiphertextAtRest(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(pool, enc)

	if _, err := pool.Exec(ctx, `DELETE FROM routing_providers WHERE name = 't5-openai'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	prov := routingDomain.ProviderConfig{
		Name:         "t5-openai",
		APIType:      "openai",
		BaseURL:      "https://api.example.com/v1",
		APIKeys:      []string{"sk-topsecret-key-1"},
		AWSAccessKeyID: "AKIAINTEGRATION00",
		AWSSecretAccessKey: "aws-integration-secret",
	}
	if err := r.UpsertProvider(ctx, prov); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}

	var rawKeys, rawAWSAccess, rawAWSSecret []byte
	if err := pool.QueryRow(ctx, `SELECT api_keys, aws_access_key_id, aws_secret_access_key
		FROM routing_providers WHERE name = 't5-openai'`).
		Scan(&rawKeys, &rawAWSAccess, &rawAWSSecret); err != nil {
		t.Fatalf("read raw row: %v", err)
	}

	if bytes.Contains(rawKeys, []byte("sk-topsecret-key-1")) {
		t.Fatal("api_keys stored in plaintext at rest")
	}
	if bytes.Contains(rawAWSAccess, []byte("AKIAINTEGRATION00")) {
		t.Fatal("aws_access_key_id stored in plaintext at rest")
	}
	if bytes.Contains(rawAWSSecret, []byte("aws-integration-secret")) {
		t.Fatal("aws_secret_access_key stored in plaintext at rest")
	}

	got, err := r.ListProviders(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	var found *routingDomain.ProviderConfig
	for i := range got {
		if got[i].Name == "t5-openai" {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatal("provider not returned")
	}
	if len(found.APIKeys) != 1 || found.APIKeys[0] != "sk-topsecret-key-1" {
		t.Errorf("api_keys read-back mismatch, got %v", found.APIKeys)
	}
	if found.AWSAccessKeyID != "AKIAINTEGRATION00" {
		t.Errorf("aws access read-back mismatch, got %q", found.AWSAccessKeyID)
	}
	if found.AWSSecretAccessKey != "aws-integration-secret" {
		t.Errorf("aws secret read-back mismatch, got %q", found.AWSSecretAccessKey)
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestPgRegistryUpsertPreservesMasked (AC-004)
func TestPgRegistryUpsertPreservesMasked(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(pool, enc)

	if _, err := pool.Exec(ctx, `DELETE FROM routing_providers WHERE name = 't5-masked'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	seed := routingDomain.ProviderConfig{
		Name:    "t5-masked",
		APIType: "openai",
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"sk-original-secret-value"},
	}
	if err := r.UpsertProvider(ctx, seed); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	masked := routingDomain.ProviderConfig{
		Name:    "t5-masked",
		APIType: "openai",
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"ski***lue"},
	}
	if err := r.UpsertProvider(ctx, masked); err != nil {
		t.Fatalf("masked upsert: %v", err)
	}

	got, err := r.ListProviders(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	for _, p := range got {
		if p.Name == "t5-masked" {
			if len(p.APIKeys) != 1 || p.APIKeys[0] != "sk-original-secret-value" {
				t.Errorf("masked upsert must preserve stored secret, got %v", p.APIKeys)
			}
			return
		}
	}
	t.Fatal("t5-masked provider not found")
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestPgRegistryReencryptLegacyRows (AC-002)
func TestPgRegistryReencryptLegacyRows(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(pool, enc)

	if _, err := pool.Exec(ctx, `DELETE FROM routing_providers WHERE name = 't5-legacy'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO routing_providers
		(name, api_type, base_url, api_keys, aws_access_key_id, aws_secret_access_key, source)
		VALUES ('t5-legacy', 'openai', 'https://api.example.com/v1',
			'["sk-legacy-plain"]', 'AKIALEGACY0000', 'legacy-aws-secret', 'yaml')`); err != nil {
		t.Fatalf("seed legacy plaintext row: %v", err)
	}

	if err := ReencryptProviderSecrets(ctx, pool, enc); err != nil {
		t.Fatalf("re-encrypt: %v", err)
	}

	var rawKeys, rawAWSAccess, rawAWSSecret []byte
	if err := pool.QueryRow(ctx, `SELECT api_keys, aws_access_key_id, aws_secret_access_key
		FROM routing_providers WHERE name = 't5-legacy'`).
		Scan(&rawKeys, &rawAWSAccess, &rawAWSSecret); err != nil {
		t.Fatalf("read raw row: %v", err)
	}
	if bytes.Contains(rawKeys, []byte("sk-legacy-plain")) {
		t.Fatal("legacy api_keys not re-encrypted")
	}
	if bytes.Contains(rawAWSAccess, []byte("AKIALEGACY0000")) {
		t.Fatal("legacy aws_access_key_id not re-encrypted")
	}
	if bytes.Contains(rawAWSSecret, []byte("legacy-aws-secret")) {
		t.Fatal("legacy aws_secret_access_key not re-encrypted")
	}

	got, err := r.ListProviders(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	for _, p := range got {
		if p.Name == "t5-legacy" {
			if len(p.APIKeys) != 1 || p.APIKeys[0] != "sk-legacy-plain" {
				t.Errorf("legacy key read-back mismatch, got %v", p.APIKeys)
			}
			if p.AWSSecretAccessKey != "legacy-aws-secret" {
				t.Errorf("legacy aws secret read-back mismatch, got %q", p.AWSSecretAccessKey)
			}
			return
		}
	}
	t.Fatal("t5-legacy provider not found")
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestPgRegistryReencryptIdempotent (AC-002)
func TestPgRegistryReencryptIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := setupPG(t, ctx)
	defer pool.Close()

	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(pool, enc)

	if _, err := pool.Exec(ctx, `DELETE FROM routing_providers WHERE name = 't5-idem'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	prov := routingDomain.ProviderConfig{
		Name:    "t5-idem",
		APIType: "openai",
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"sk-idempotent-secret"},
	}
	if err := r.UpsertProvider(ctx, prov); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := ReencryptProviderSecrets(ctx, pool, enc); err != nil {
		t.Fatalf("first re-encrypt: %v", err)
	}
	if err := ReencryptProviderSecrets(ctx, pool, enc); err != nil {
		t.Fatalf("second re-encrypt: %v", err)
	}

	got, err := r.ListProviders(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	for _, p := range got {
		if p.Name == "t5-idem" {
			if len(p.APIKeys) != 1 || p.APIKeys[0] != "sk-idempotent-secret" {
				t.Errorf("idempotent re-encrypt changed value, got %v", p.APIKeys)
			}
			return
		}
	}
	t.Fatal("t5-idem provider not found")
}
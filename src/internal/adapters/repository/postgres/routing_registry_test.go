package postgres

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

func pgTestKey(t *testing.T) *crypto.Encryptor {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	enc, err := crypto.New(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return enc
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestSealOpenJSONRoundTrip (AC-002)
func TestSealOpenJSONRoundTrip(t *testing.T) {
	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(nil, enc)

	sealed, err := r.sealJSON([]string{"sk-secret-1", "sk-secret-2"})
	if err != nil {
		t.Fatalf("sealJSON: %v", err)
	}
	if !bytes.HasPrefix(sealed, []byte(`"enc:`)) {
		t.Errorf("expected JSON-string sealed value, got %s", sealed)
	}
	var plaintextSecret string
	if err := json.Unmarshal(sealed, &plaintextSecret); err != nil {
		t.Fatalf("unmarshal sealed: %v", err)
	}
	if !bytes.HasPrefix([]byte(plaintextSecret), []byte(encPrefix)) {
		t.Errorf("expected enc: prefix, got %q", plaintextSecret)
	}
	if bytes.Contains([]byte(plaintextSecret), []byte("sk-secret-1")) {
		t.Error("ciphertext must not contain plaintext key material")
	}

	var keysOut []string
	if err := r.openJSON(sealed, &keysOut); err != nil {
		t.Fatalf("openJSON: %v", err)
	}
	if len(keysOut) != 2 || keysOut[0] != "sk-secret-1" || keysOut[1] != "sk-secret-2" {
		t.Errorf("round-trip mismatch, got %v", keysOut)
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestOpenJSONLegacyPlaintextPassesThrough (AC-002)
func TestOpenJSONLegacyPlaintextPassesThrough(t *testing.T) {
	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(nil, enc)

	raw := []byte(`["sk-legacy-plaintext"]`)
	var keys []string
	if err := r.openJSON(raw, &keys); err != nil {
		t.Fatalf("openJSON legacy: %v", err)
	}
	if len(keys) != 1 || keys[0] != "sk-legacy-plaintext" {
		t.Errorf("legacy plaintext must pass through unchanged, got %v", keys)
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestOpenTextRoundTripAndLegacy (AC-002)
func TestOpenTextRoundTripAndLegacy(t *testing.T) {
	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(nil, enc)

	sealed, err := r.sealText("AWS-SECRET-VALUE")
	if err != nil {
		t.Fatalf("sealText: %v", err)
	}
	got, err := r.openText(sealed)
	if err != nil {
		t.Fatalf("openText sealed: %v", err)
	}
	if got != "AWS-SECRET-VALUE" {
		t.Errorf("sealed round-trip mismatch, got %q", got)
	}

	legacy, err := r.openText([]byte("legacy-plain-aws-secret"))
	if err != nil {
		t.Fatalf("openText legacy: %v", err)
	}
	if legacy != "legacy-plain-aws-secret" {
		t.Errorf("legacy text must pass through, got %q", legacy)
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestPreserveMaskedSecrets (AC-004)
func TestPreserveMaskedSecrets(t *testing.T) {
	existing := routingDomain.ProviderConfig{
		APIKeys:            []string{"sk-original-1", "sk-original-2"},
		AWSAccessKeyID:     "AKIAORIGINAL0000",
		AWSSecretAccessKey: "orig-secret-aws",
	}

	submitted := routingDomain.ProviderConfig{
		APIKeys:            []string{"ski***789", "sk-other-2"},
		AWSAccessKeyID:     "AKI***0000",
		AWSSecretAccessKey: "",
	}
	changed := preserveMaskedSecrets(&submitted, existing)
	if !changed {
		t.Fatal("expected preservation to report a change")
	}
	if submitted.APIKeys[0] != "sk-original-1" {
		t.Errorf("masked key should be preserved, got %q", submitted.APIKeys[0])
	}
	if submitted.APIKeys[1] != "sk-other-2" {
		t.Errorf("non-masked key should pass through, got %q", submitted.APIKeys[1])
	}
	if submitted.AWSAccessKeyID != "AKIAORIGINAL0000" {
		t.Errorf("masked aws access key should be preserved, got %q", submitted.AWSAccessKeyID)
	}
	if submitted.AWSSecretAccessKey != "orig-secret-aws" {
		t.Errorf("empty aws secret should keep existing value, got %q", submitted.AWSSecretAccessKey)
	}
}

// @sk-test provider-model-registry#T1.4: existing providers are never re-seeded (AC-006)
func TestSeedPlanSkipsExistingProviders(t *testing.T) {
	existing := map[string]struct{}{"keep": {}}
	providers := []routingDomain.ProviderConfig{
		{Name: "keep", Models: []string{"must-not-seed"}},
		{Name: "new", Models: []string{"m1"}},
		{Name: "", Models: []string{"ignored"}},
	}

	insert, models := seedPlan(providers, existing)

	if len(insert) != 1 || insert[0].Name != "new" {
		t.Fatalf("insert = %+v, want only [new]", insert)
	}
	if _, ok := models["keep"]; ok {
		t.Error("existing provider must not get seeded models")
	}
	if got := models["new"]; len(got) != 1 || got[0] != "m1" {
		t.Errorf("models[new] = %v, want [m1]", got)
	}
}

// @sk-test provider-model-registry#T1.4: declared models normalize to catalog routes (AC-001)
func TestProviderModelRoutesNormalizes(t *testing.T) {
	got := providerModelRoutes(routingDomain.ProviderConfig{
		Name:   "groq",
		Models: []string{" m1 ", "m1", "", "groq/*", "m2"},
	})
	want := []string{"m1", "m2"}
	if len(got) != len(want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("routes = %v, want %v", got, want)
		}
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestMaskSecretShortAndEmpty (AC-003)
func TestMaskSecretShortAndEmpty(t *testing.T) {
	if isMaskedLiteral("short") {
		t.Error("real short values are not masked literals")
	}
	if !isMaskedLiteral("art***word") {
		t.Error("value with *** marker must be detected as masked")
	}
	if !isMaskedLiteral("") {
		t.Error("empty value is a masked/keep literal")
	}
}

// @sk-test 403-key-at-rest-encryption#T5.1: TestSealTextEmptyStaysNonNull (AC-002)
func TestSealTextEmptyStaysNonNull(t *testing.T) {
	enc := pgTestKey(t)
	r := NewPostgresRegistryRepositoryWithEncryptor(nil, enc)

	sealed, err := r.sealText("")
	if err != nil {
		t.Fatalf("sealText empty: %v", err)
	}
	if sealed == nil {
		t.Fatal("sealText empty must not return nil (would insert SQL NULL into NOT NULL columns)")
	}
	if len(sealed) != 0 {
		t.Errorf("expected empty slice, got %q", sealed)
	}
}

// @sk-test model-aliases-weighted-lb#T1.6: incomplete alias entries are dropped before seeding (AC-009)
func TestAliasesForSeed(t *testing.T) {
	got := aliasesForSeed([]routingDomain.AliasConfig{
		{Tenant: "acme", Alias: "m", Target: "target"},
		{Tenant: "", Alias: "m", Target: "target"},
		{Tenant: "acme", Alias: "", Target: "target"},
		{Tenant: "acme", Alias: "m", Target: ""},
	})
	if len(got) != 1 || got[0].Alias != "m" || got[0].Target != "target" {
		t.Fatalf("aliasesForSeed = %+v, want one complete alias", got)
	}
}

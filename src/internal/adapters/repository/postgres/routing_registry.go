package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/internal/infra/crypto"
)

// encPrefix marks an at-rest encrypted value. JSONB secret columns hold this
// prefix inside a JSON string ("enc:<base64>"), TEXT secret columns hold it raw.
const encPrefix = "enc:"

// @sk-task 403-key-at-rest-encryption#T2.1: PostgresRegistryRepository with encryptor (AC-001, AC-008)
//
// PostgresRegistryRepository persists routing providers and model routes.
// Secret-bearing columns (api_keys, additional_headers, aws credentials) are
// sealed with AES-256-GCM at the repository boundary and opened on read.
type PostgresRegistryRepository struct {
	pool *pgxpool.Pool
	enc  *crypto.Encryptor
}

func NewPostgresRegistryRepository(pool *pgxpool.Pool) *PostgresRegistryRepository {
	// @sk-task 403-key-at-rest-encryption#T2.1: Auto-configure encryptor from env (AC-008)
	r := &PostgresRegistryRepository{pool: pool}
	if key := os.Getenv(config.KeysKeyEnvVar); key != "" {
		if enc, err := crypto.New(key); err == nil {
			r.enc = enc
		}
	}
	return r
}

// NewPostgresRegistryRepositoryWithEncryptor builds the repository with an
// explicit encryptor, used by tests to control the at-rest key.
func NewPostgresRegistryRepositoryWithEncryptor(pool *pgxpool.Pool, enc *crypto.Encryptor) *PostgresRegistryRepository {
	return &PostgresRegistryRepository{pool: pool, enc: enc}
}

// sealBytes encrypts plaintext and returns "enc:<base64(nonce||ct)>".
func (r *PostgresRegistryRepository) sealBytes(plain []byte) ([]byte, error) {
	if r.enc == nil {
		return nil, fmt.Errorf("routing registry encryption not configured (set %s)", config.KeysKeyEnvVar)
	}
	sealed, err := r.enc.Encrypt(plain)
	if err != nil {
		return nil, fmt.Errorf("encrypt provider secret: %w", err)
	}
	return []byte(encPrefix + base64.StdEncoding.EncodeToString(sealed)), nil
}

// openBytes decrypts a stored secret. Legacy plaintext values (pre-encryption
// rows) pass through unchanged so a missing decrypt never breaks reads.
// Returns the plaintext bytes and whether the value was sealed.
func (r *PostgresRegistryRepository) openBytes(stored []byte, isJSONB bool) ([]byte, bool, error) {
	var payload []byte
	if isJSONB {
		if len(stored) == 0 {
			return stored, false, nil
		}
		if stored[0] == '"' {
			var s string
			if err := json.Unmarshal(stored, &s); err != nil {
				return nil, false, fmt.Errorf("parse sealed provider secret: %w", err)
			}
			payload = []byte(s)
		} else {
			return stored, false, nil
		}
	} else {
		payload = stored
	}
	if !bytes.HasPrefix(payload, []byte(encPrefix)) {
		return payload, false, nil
	}
	if r.enc == nil {
		return nil, false, fmt.Errorf("routing registry encryption not configured (set %s)", config.KeysKeyEnvVar)
	}
	sealed, err := base64.StdEncoding.DecodeString(string(bytes.TrimPrefix(payload, []byte(encPrefix))))
	if err != nil {
		return nil, false, fmt.Errorf("decode sealed provider secret: %w", err)
	}
	plain, err := r.enc.Decrypt(sealed)
	if err != nil {
		return nil, false, fmt.Errorf("decrypt provider secret: %w", err)
	}
	return plain, true, nil
}

// sealJSON seals a JSON-marshaled value for a JSONB column (JSON string form).
func (r *PostgresRegistryRepository) sealJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	sealed, err := r.sealBytes(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(sealed))
}

// openJSON opens a JSONB secret column into v, supporting legacy plaintext.
func (r *PostgresRegistryRepository) openJSON(raw []byte, v any) error {
	plain, _, err := r.openBytes(raw, true)
	if err != nil {
		return err
	}
	if len(plain) == 0 {
		return nil
	}
	return json.Unmarshal(plain, v)
}

// sealText seals a TEXT secret column; empty stays as an empty (non-NULL) value
// because the schema declares these columns NOT NULL DEFAULT ”.
func (r *PostgresRegistryRepository) sealText(s string) ([]byte, error) {
	if s == "" {
		return []byte{}, nil
	}
	return r.sealBytes([]byte(s))
}

// openText opens a TEXT secret column; legacy plaintext passes through.
func (r *PostgresRegistryRepository) openText(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	plain, _, err := r.openBytes(raw, false)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// @sk-task 403-key-at-rest-encryption#T2.3: Preserve stored secrets on masked/empty upsert (AC-004)
//
// isMaskedLiteral reports whether a submitted secret value is the masked display
// form (contains "***") rather than a real secret.
func isMaskedLiteral(s string) bool {
	return s == "" || containsMaskMarker(s)
}

func containsMaskMarker(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '*' && s[i+1] == '*' && s[i+2] == '*' {
			return true
		}
	}
	return false
}

// providerSecrets reads the current decrypted secret fields for a provider.
func (r *PostgresRegistryRepository) providerSecrets(ctx context.Context, q querier, name string) (routingDomain.ProviderConfig, error) {
	var p routingDomain.ProviderConfig
	var rawKeys, rawHeaders, rawAWSAccess, rawAWSSecret []byte
	err := q.QueryRow(ctx, `
		SELECT api_keys, additional_headers, aws_access_key_id, aws_secret_access_key
		FROM routing_providers WHERE name = $1`, name).
		Scan(&rawKeys, &rawHeaders, &rawAWSAccess, &rawAWSSecret)
	if err != nil {
		return p, err
	}
	if err := r.openJSON(rawKeys, &p.APIKeys); err != nil {
		return p, err
	}
	if err := r.openJSON(rawHeaders, &p.AdditionalHeaders); err != nil {
		return p, err
	}
	if p.AWSAccessKeyID, err = r.openText(rawAWSAccess); err != nil {
		return p, err
	}
	if p.AWSSecretAccessKey, err = r.openText(rawAWSSecret); err != nil {
		return p, err
	}
	return p, nil
}

// preserveMaskedSecrets replaces masked/empty submitted secret fields with the
// stored decrypted values. It reports whether any preservation occurred.
// An empty submitted value is treated as "keep the current secret" (DEC-003).
func preserveMaskedSecrets(submitted *routingDomain.ProviderConfig, existing routingDomain.ProviderConfig) bool {
	changed := false
	for i, k := range submitted.APIKeys {
		if isMaskedLiteral(k) && len(existing.APIKeys) > i {
			submitted.APIKeys[i] = existing.APIKeys[i]
			changed = true
		}
	}
	if isMaskedLiteral(submitted.AWSAccessKeyID) && existing.AWSAccessKeyID != "" {
		submitted.AWSAccessKeyID = existing.AWSAccessKeyID
		changed = true
	}
	if isMaskedLiteral(submitted.AWSSecretAccessKey) && existing.AWSSecretAccessKey != "" {
		submitted.AWSSecretAccessKey = existing.AWSSecretAccessKey
		changed = true
	}
	return changed
}

func (r *PostgresRegistryRepository) ListProviders(ctx context.Context) ([]routingDomain.ProviderConfig, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx, `
		SELECT name, api_type, base_url, health_endpoint, timeout, priority, api_keys,
			auth_scheme, auth_header, auth_prefix, additional_headers, proxy_url,
			aws_region, aws_access_key_id, aws_secret_access_key, source
		FROM routing_providers
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var out []routingDomain.ProviderConfig
	for rows.Next() {
		var p routingDomain.ProviderConfig
		var rawKeys, rawHeaders, rawAWSAccess, rawAWSSecret []byte
		var source string
		if err := rows.Scan(&p.Name, &p.APIType, &p.BaseURL, &p.HealthEndpoint, &p.Timeout, &p.Priority,
			&rawKeys, &p.AuthScheme, &p.AuthHeader, &p.AuthPrefix, &rawHeaders, &p.ProxyURL,
			&p.AWSRegion, &rawAWSAccess, &rawAWSSecret, &source); err != nil {
			return nil, err
		}
		p.Source = source
		if err := r.openJSON(rawKeys, &p.APIKeys); err != nil {
			return nil, fmt.Errorf("provider %s: api_keys: %w", p.Name, err)
		}
		if err := r.openJSON(rawHeaders, &p.AdditionalHeaders); err != nil {
			return nil, fmt.Errorf("provider %s: additional_headers: %w", p.Name, err)
		}
		if p.AWSAccessKeyID, err = r.openText(rawAWSAccess); err != nil {
			return nil, fmt.Errorf("provider %s: aws_access_key_id: %w", p.Name, err)
		}
		if p.AWSSecretAccessKey, err = r.openText(rawAWSSecret); err != nil {
			return nil, fmt.Errorf("provider %s: aws_secret_access_key: %w", p.Name, err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	if out == nil {
		return []routingDomain.ProviderConfig{}, nil
	}
	return out, nil
}

// @sk-task 403-key-at-rest-encryption#T2.2: ReencryptLegacyProviderSecrets seals legacy plaintext rows (AC-002)
//
// ReencryptProviderSecrets walks routing_providers and seals any secret column
// still stored as legacy plaintext (pre-encryption rows). Already-sealed and
// empty values are left untouched, so the operation is idempotent. It must run
// after migrations while no writes are in-flight.
func ReencryptProviderSecrets(ctx context.Context, pool *pgxpool.Pool, enc *crypto.Encryptor) error {
	r := NewPostgresRegistryRepositoryWithEncryptor(pool, enc)
	q := getQuerier(ctx, pool)

	rows, err := q.Query(ctx, `
		SELECT name, api_keys, additional_headers, aws_access_key_id, aws_secret_access_key
		FROM routing_providers
		ORDER BY name`)
	if err != nil {
		return fmt.Errorf("re-encrypt list providers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var rawKeys, rawHeaders, rawAWSAccess, rawAWSSecret []byte
		if err := rows.Scan(&name, &rawKeys, &rawHeaders, &rawAWSAccess, &rawAWSSecret); err != nil {
			return err
		}

		needUpdate := false
		if _, sealed, err := r.openBytes(rawKeys, true); err != nil {
			return fmt.Errorf("re-encrypt provider %s api_keys: %w", name, err)
		} else if !sealed {
			needUpdate = true
		}
		if _, sealed, err := r.openBytes(rawHeaders, true); err != nil {
			return fmt.Errorf("re-encrypt provider %s additional_headers: %w", name, err)
		} else if !sealed {
			needUpdate = true
		}
		if len(rawAWSAccess) > 0 {
			if _, sealed, err := r.openBytes(rawAWSAccess, false); err != nil {
				return fmt.Errorf("re-encrypt provider %s aws_access_key_id: %w", name, err)
			} else if !sealed {
				needUpdate = true
			}
		}
		if len(rawAWSSecret) > 0 {
			if _, sealed, err := r.openBytes(rawAWSSecret, false); err != nil {
				return fmt.Errorf("re-encrypt provider %s aws_secret_access_key: %w", name, err)
			} else if !sealed {
				needUpdate = true
			}
		}
		if !needUpdate {
			continue
		}

		var p routingDomain.ProviderConfig
		p.Name = name
		if err := r.openJSON(rawKeys, &p.APIKeys); err != nil {
			return fmt.Errorf("re-encrypt provider %s: %w", name, err)
		}
		if err := r.openJSON(rawHeaders, &p.AdditionalHeaders); err != nil {
			return fmt.Errorf("re-encrypt provider %s: %w", name, err)
		}
		if p.AWSAccessKeyID, err = r.openText(rawAWSAccess); err != nil {
			return fmt.Errorf("re-encrypt provider %s: %w", name, err)
		}
		if p.AWSSecretAccessKey, err = r.openText(rawAWSSecret); err != nil {
			return fmt.Errorf("re-encrypt provider %s: %w", name, err)
		}
		if err := r.updateSecrets(ctx, p); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("re-encrypt rows iteration: %w", err)
	}
	return nil
}

// updateSecrets writes back sealed secret columns for an existing provider
// without touching non-secret attributes or provenance.
func (r *PostgresRegistryRepository) updateSecrets(ctx context.Context, p routingDomain.ProviderConfig) error {
	q := getQuerier(ctx, r.pool)

	keys, err := r.sealJSON(p.APIKeys)
	if err != nil {
		return fmt.Errorf("re-encrypt provider %s: %w", p.Name, err)
	}
	headers, err := r.sealJSON(p.AdditionalHeaders)
	if err != nil {
		return fmt.Errorf("re-encrypt provider %s: %w", p.Name, err)
	}
	awsAccess, err := r.sealText(p.AWSAccessKeyID)
	if err != nil {
		return fmt.Errorf("re-encrypt provider %s: %w", p.Name, err)
	}
	awsSecret, err := r.sealText(p.AWSSecretAccessKey)
	if err != nil {
		return fmt.Errorf("re-encrypt provider %s: %w", p.Name, err)
	}
	_, err = q.Exec(ctx, `
		UPDATE routing_providers
		SET api_keys = $2, additional_headers = $3,
			aws_access_key_id = $4, aws_secret_access_key = $5,
			updated_at = now()
		WHERE name = $1`, p.Name, keys, headers, awsAccess, awsSecret)
	if err != nil {
		return fmt.Errorf("re-encrypt provider %s: write: %w", p.Name, err)
	}
	return nil
}

func (r *PostgresRegistryRepository) ListRules(ctx context.Context) ([]routingDomain.RuleConfig, error) {
	q := getQuerier(ctx, r.pool)

	rows, err := q.Query(ctx, `
		SELECT model, tenant, providers, source
		FROM routing_model_routes
		ORDER BY model`)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	type flat struct {
		model     string
		tenant    string
		providers []string
		source    string
	}
	var flats []flat
	for rows.Next() {
		var f flat
		var rawProv []byte
		if err := rows.Scan(&f.model, &f.tenant, &rawProv, &f.source); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawProv, &f.providers)
		flats = append(flats, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Group rows by tenant into rules.
	ruleByTenant := map[string]*routingDomain.RuleConfig{}
	var order []string
	for _, f := range flats {
		r, ok := ruleByTenant[f.tenant]
		if !ok {
			r = &routingDomain.RuleConfig{Tenant: f.tenant, Source: f.source}
			ruleByTenant[f.tenant] = r
			order = append(order, f.tenant)
		}
		r.Routes = append(r.Routes, routingDomain.RouteConfig{Model: f.model, Providers: f.providers})
	}

	out := make([]routingDomain.RuleConfig, 0, len(order))
	for _, tenant := range order {
		out = append(out, *ruleByTenant[tenant])
	}
	return out, nil
}

func (r *PostgresRegistryRepository) UpsertProvider(ctx context.Context, p routingDomain.ProviderConfig) error {
	q := getQuerier(ctx, r.pool)

	// @sk-task 403-key-at-rest-encryption#T2.3: Preserve stored secrets when the form submits masked or empty values (AC-004)
	if existing, err := r.providerSecrets(ctx, q, p.Name); err == nil {
		preserveMaskedSecrets(&p, existing)
	}

	keys, err := r.sealJSON(p.APIKeys)
	if err != nil {
		return fmt.Errorf("provider %s: seal api_keys: %w", p.Name, err)
	}
	headers, err := r.sealJSON(p.AdditionalHeaders)
	if err != nil {
		return fmt.Errorf("provider %s: seal additional_headers: %w", p.Name, err)
	}
	awsAccess, err := r.sealText(p.AWSAccessKeyID)
	if err != nil {
		return fmt.Errorf("provider %s: seal aws_access_key_id: %w", p.Name, err)
	}
	awsSecret, err := r.sealText(p.AWSSecretAccessKey)
	if err != nil {
		return fmt.Errorf("provider %s: seal aws_secret_access_key: %w", p.Name, err)
	}
	_, err = q.Exec(ctx, `
		INSERT INTO routing_providers
			(name, api_type, base_url, health_endpoint, timeout, priority, api_keys,
			 auth_scheme, auth_header, auth_prefix, additional_headers, proxy_url,
			 aws_region, aws_access_key_id, aws_secret_access_key, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 'ui')
		ON CONFLICT (name) DO UPDATE SET
			api_type = EXCLUDED.api_type,
			base_url = EXCLUDED.base_url,
			health_endpoint = EXCLUDED.health_endpoint,
			timeout = EXCLUDED.timeout,
			priority = EXCLUDED.priority,
			api_keys = EXCLUDED.api_keys,
			auth_scheme = EXCLUDED.auth_scheme,
			auth_header = EXCLUDED.auth_header,
			auth_prefix = EXCLUDED.auth_prefix,
			additional_headers = EXCLUDED.additional_headers,
			proxy_url = EXCLUDED.proxy_url,
			aws_region = EXCLUDED.aws_region,
			aws_access_key_id = EXCLUDED.aws_access_key_id,
			aws_secret_access_key = EXCLUDED.aws_secret_access_key,
			source = 'ui',
			updated_at = now()`,
		p.Name, p.APIType, p.BaseURL, p.HealthEndpoint, p.Timeout, p.Priority, keys,
		p.AuthScheme, p.AuthHeader, p.AuthPrefix, headers, p.ProxyURL,
		p.AWSRegion, awsAccess, awsSecret)
	if err != nil {
		return fmt.Errorf("upsert provider %s: %w", p.Name, err)
	}
	return nil
}

func (r *PostgresRegistryRepository) DeleteProvider(ctx context.Context, name string) error {
	q := getQuerier(ctx, r.pool)

	// Strip references from routes before dropping the provider row.
	_, _ = q.Exec(ctx, `
		UPDATE routing_model_routes
		SET providers = (SELECT jsonb_agg(p) FROM jsonb_array_elements_text(providers) AS t(p) WHERE p <> $1)
		WHERE $1::TEXT = ANY (ARRAY(SELECT jsonb_array_elements_text(providers)))`, name)

	tag, err := q.Exec(ctx, `DELETE FROM routing_providers WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete provider %s: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return routingDomain.ErrNotFound
	}
	return nil
}

func (r *PostgresRegistryRepository) UpsertRoute(ctx context.Context, tenant, model string, providers []string) error {
	q := getQuerier(ctx, r.pool)

	prov, err := json.Marshal(providers)
	if err != nil {
		return fmt.Errorf("marshal providers: %w", err)
	}
	_, err = q.Exec(ctx, `
		INSERT INTO routing_model_routes (model, tenant, providers, source)
		VALUES ($1, $2, $3, 'ui')
		ON CONFLICT (model, tenant) DO UPDATE SET
			providers = EXCLUDED.providers,
			source = 'ui',
			updated_at = now()`,
		model, tenant, prov)
	if err != nil {
		return fmt.Errorf("upsert route %s/%s: %w", tenant, model, err)
	}
	return nil
}

func (r *PostgresRegistryRepository) DeleteRoute(ctx context.Context, tenant, model string) error {
	q := getQuerier(ctx, r.pool)

	tag, err := q.Exec(ctx, `DELETE FROM routing_model_routes WHERE tenant = $1 AND model = $2`, tenant, model)
	if err != nil {
		return fmt.Errorf("delete route %s/%s: %w", tenant, model, err)
	}
	if tag.RowsAffected() == 0 {
		return routingDomain.ErrNotFound
	}
	return nil
}

// @sk-task provider-model-registry#T1.2: per-provider idempotent seeding (AC-001, AC-006)
//
// SeedFromYAML inserts YAML-declared providers that are not already stored, and
// materializes each new provider's declared models as global routes. A provider
// that already exists (yaml- or ui-managed) is never modified, so UI edits and
// prior YAML seeds survive a restart. Returns true when anything was seeded.
func (r *PostgresRegistryRepository) SeedFromYAML(ctx context.Context, providers []routingDomain.ProviderConfig, rules []routingDomain.RuleConfig) (bool, error) {
	q := getQuerier(ctx, r.pool)

	existing, err := r.providerNames(ctx, q)
	if err != nil {
		return false, err
	}
	toInsert, modelRoutes := seedPlan(providers, existing)

	seeded := false
	for _, p := range toInsert {
		keys, err := r.sealJSON(p.APIKeys)
		if err != nil {
			return false, fmt.Errorf("seed provider %s: seal api_keys: %w", p.Name, err)
		}
		headers, err := r.sealJSON(p.AdditionalHeaders)
		if err != nil {
			return false, fmt.Errorf("seed provider %s: seal additional_headers: %w", p.Name, err)
		}
		awsAccess, err := r.sealText(p.AWSAccessKeyID)
		if err != nil {
			return false, fmt.Errorf("seed provider %s: seal aws_access_key_id: %w", p.Name, err)
		}
		awsSecret, err := r.sealText(p.AWSSecretAccessKey)
		if err != nil {
			return false, fmt.Errorf("seed provider %s: seal aws_secret_access_key: %w", p.Name, err)
		}
		tag, err := q.Exec(ctx, `
			INSERT INTO routing_providers
				(name, api_type, base_url, health_endpoint, timeout, priority, api_keys, auth_scheme, auth_header, auth_prefix, additional_headers, proxy_url, aws_region, aws_access_key_id, aws_secret_access_key, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 'yaml')
			ON CONFLICT (name) DO NOTHING`,
			p.Name, p.APIType, p.BaseURL, p.HealthEndpoint, p.Timeout, p.Priority, keys,
			p.AuthScheme, p.AuthHeader, p.AuthPrefix, headers, p.ProxyURL,
			p.AWSRegion, awsAccess, awsSecret)
		if err != nil {
			return false, fmt.Errorf("seed provider %s: %w", p.Name, err)
		}
		if tag.RowsAffected() == 0 {
			// A concurrent writer created it; never mutate it.
			continue
		}
		seeded = true

		for _, model := range modelRoutes[p.Name] {
			prov, err := json.Marshal([]string{p.Name})
			if err != nil {
				return false, fmt.Errorf("seed model %s/%s: marshal providers: %w", p.Name, model, err)
			}
			if _, err := q.Exec(ctx, `
				INSERT INTO routing_model_routes (model, tenant, providers, source)
				VALUES ($1, $2, $3, 'yaml')
				ON CONFLICT (model, tenant) DO NOTHING`,
				model, routingDomain.GlobalTenant, prov); err != nil {
				return false, fmt.Errorf("seed model %s/%s: %w", p.Name, model, err)
			}
		}
	}

	for _, rule := range rules {
		for _, rt := range rule.Routes {
			prov, err := json.Marshal(rt.Providers)
			if err != nil {
				return false, fmt.Errorf("marshal providers: %w", err)
			}
			tag, err := q.Exec(ctx, `
				INSERT INTO routing_model_routes (model, tenant, providers, source)
				VALUES ($1, $2, $3, 'yaml')
				ON CONFLICT (model, tenant) DO NOTHING`,
				rt.Model, rule.Tenant, prov)
			if err != nil {
				return false, fmt.Errorf("seed route %s/%s: %w", rule.Tenant, rt.Model, err)
			}
			if tag.RowsAffected() > 0 {
				seeded = true
			}
		}
	}
	return seeded, nil
}

// providerNames returns the set of provider names currently stored.
func (r *PostgresRegistryRepository) providerNames(ctx context.Context, q querier) (map[string]struct{}, error) {
	rows, err := q.Query(ctx, `SELECT name FROM routing_providers`)
	if err != nil {
		return nil, fmt.Errorf("list provider names: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// seedPlan selects providers absent from existing and, for each, the global
// model routes to materialize. Existing providers are never returned, so seeding
// cannot mutate a provider that is already yaml- or ui-managed.
func seedPlan(providers []routingDomain.ProviderConfig, existing map[string]struct{}) (insert []routingDomain.ProviderConfig, models map[string][]string) {
	models = map[string][]string{}
	for _, p := range providers {
		if p.Name == "" {
			continue
		}
		if _, ok := existing[p.Name]; ok {
			continue
		}
		insert = append(insert, p)
		if r := providerModelRoutes(p); len(r) > 0 {
			models[p.Name] = r
		}
	}
	return insert, models
}

// providerModelRoutes normalizes a provider's declared models into global routes:
// trimmed, de-duplicated, and excluding wildcard patterns (patterns are routes,
// not catalog entries).
func providerModelRoutes(p routingDomain.ProviderConfig) []string {
	seen := make(map[string]struct{}, len(p.Models))
	out := make([]string, 0, len(p.Models))
	for _, m := range p.Models {
		m = strings.TrimSpace(m)
		if m == "" || strings.Contains(m, "*") {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

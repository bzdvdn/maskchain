package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	routingDomain "github.com/bzdvdn/maskchain/src/internal/domain/routing"
)

// @sk-task 150-admin-routing-crud#T1.3: PostgresRegistryRepository (AC-001)
//
// PostgresRegistryRepository persists routing providers and model routes.
type PostgresRegistryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRegistryRepository(pool *pgxpool.Pool) *PostgresRegistryRepository {
	return &PostgresRegistryRepository{pool: pool}
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
		var rawKeys, rawHeaders []byte
		var source string
		if err := rows.Scan(&p.Name, &p.APIType, &p.BaseURL, &p.HealthEndpoint, &p.Timeout, &p.Priority,
			&rawKeys, &p.AuthScheme, &p.AuthHeader, &p.AuthPrefix, &rawHeaders, &p.ProxyURL,
			&p.AWSRegion, &p.AWSAccessKeyID, &p.AWSSecretAccessKey, &source); err != nil {
			return nil, err
		}
		p.Source = source
		_ = json.Unmarshal(rawKeys, &p.APIKeys)
		_ = json.Unmarshal(rawHeaders, &p.AdditionalHeaders)
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

	keys, err := json.Marshal(p.APIKeys)
	if err != nil {
		return fmt.Errorf("marshal keys: %w", err)
	}
	headers, err := json.Marshal(p.AdditionalHeaders)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
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
		p.AWSRegion, p.AWSAccessKeyID, p.AWSSecretAccessKey)
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

func (r *PostgresRegistryRepository) SeedFromYAML(ctx context.Context, providers []routingDomain.ProviderConfig, rules []routingDomain.RuleConfig) (bool, error) {
	q := getQuerier(ctx, r.pool)

	var count int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM routing_providers`).Scan(&count); err != nil {
		return false, fmt.Errorf("count providers: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	for _, p := range providers {
		keys, err := json.Marshal(p.APIKeys)
		if err != nil {
			return false, fmt.Errorf("marshal keys: %w", err)
		}
		headers, err := json.Marshal(p.AdditionalHeaders)
		if err != nil {
			return false, fmt.Errorf("marshal headers: %w", err)
		}
		_, err = q.Exec(ctx, `
			INSERT INTO routing_providers
				(name, api_type, base_url, health_endpoint, timeout, priority, api_keys, auth_scheme, auth_header, auth_prefix, additional_headers, proxy_url, aws_region, aws_access_key_id, aws_secret_access_key, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 'yaml')`,
			p.Name, p.APIType, p.BaseURL, p.HealthEndpoint, p.Timeout, p.Priority, keys,
			p.AuthScheme, p.AuthHeader, p.AuthPrefix, headers, p.ProxyURL,
			p.AWSRegion, p.AWSAccessKeyID, p.AWSSecretAccessKey)
		if err != nil {
			return false, fmt.Errorf("seed provider %s: %w", p.Name, err)
		}
	}

	for _, rule := range rules {
		for _, rt := range rule.Routes {
			prov, err := json.Marshal(rt.Providers)
			if err != nil {
				return false, fmt.Errorf("marshal providers: %w", err)
			}
			_, err = q.Exec(ctx, `
				INSERT INTO routing_model_routes (model, tenant, providers, source)
				VALUES ($1, $2, $3, 'yaml')`,
				rt.Model, rule.Tenant, prov)
			if err != nil {
				return false, fmt.Errorf("seed route %s/%s: %w", rule.Tenant, rt.Model, err)
			}
		}
	}
	return true, nil
}

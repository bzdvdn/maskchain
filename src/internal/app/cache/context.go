package cacheapp

import "context"

// @sk-task semantic-cache-masked#T3.1: tenant tag for degraded metrics (AC-006)
//
// tenantCtxKey carries the tenant slug on the request context so embedders can
// attribute degradation metrics.
type tenantCtxKey struct{}

// WithTenant returns ctx tagged with the tenant slug.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenantID)
}

// TenantFrom returns the tenant slug attached by WithTenant, or "unknown".
func TenantFrom(ctx context.Context) string {
	if v, ok := ctx.Value(tenantCtxKey{}).(string); ok && v != "" {
		return v
	}
	return "unknown"
}

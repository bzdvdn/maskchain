package routing

// @sk-task model-aliases-weighted-lb#T1.2: tenant-scoped model alias entity (AC-001, AC-002)
//
// Alias maps a requested model name to the model that should actually be routed
// for a tenant. Resolution is single-hop: an alias target is never re-aliased.
type Alias struct {
	Tenant string
	Alias  string
	Target string
	Source string
}

func NewAlias(tenant, alias, target, source string) Alias {
	return Alias{Tenant: tenant, Alias: alias, Target: target, Source: source}
}

// @sk-task model-aliases-weighted-lb#T2.1: single-hop alias resolution (AC-001, AC-002)
//
// ResolveAlias looks up the target for a requested model for a tenant: the
// tenant map first, then the reserved global tenant map. It returns the resolved
// model and whether an alias was applied. An empty target is ignored.
func ResolveAlias(byTenant map[string]map[string]string, tenant, model string) (string, bool) {
	if m, ok := byTenant[tenant]; ok {
		if target, ok := m[model]; ok && target != "" {
			return target, true
		}
	}
	if tenant != GlobalTenant {
		if m, ok := byTenant[GlobalTenant]; ok {
			if target, ok := m[model]; ok && target != "" {
				return target, true
			}
		}
	}
	return model, false
}

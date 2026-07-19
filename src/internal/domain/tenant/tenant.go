package tenant

// @sk-task 80-tenant-isolation#T1.1: Tenant aggregate root (AC-001, AC-002, AC-003, AC-004)
type Tenant struct {
	slug        string
	name        string
	profileSlug string
	apiKeys     []APIKey
	authHeader  string
	authScheme  string
}

func NewTenant(slug, name, profileSlug string, apiKeys []APIKey, authHeader, authScheme string) *Tenant {
	t := &Tenant{
		slug:        slug,
		name:        name,
		profileSlug: profileSlug,
		apiKeys:     apiKeys,
	}
	if authHeader == "" {
		t.authHeader = "X-Mask-Authorization"
	} else {
		t.authHeader = authHeader
	}
	if authScheme == "" {
		t.authScheme = "raw"
	} else {
		t.authScheme = authScheme
	}
	return t
}

func (t *Tenant) Slug() string        { return t.slug }
func (t *Tenant) Name() string        { return t.name }
func (t *Tenant) ProfileSlug() string { return t.profileSlug }
func (t *Tenant) APIKeys() []APIKey   { return t.apiKeys }
func (t *Tenant) AuthHeader() string  { return t.authHeader }
func (t *Tenant) AuthScheme() string  { return t.authScheme }

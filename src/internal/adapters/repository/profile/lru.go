package profilerepo

import (
	"github.com/hashicorp/golang-lru/v2"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
)

// @sk-task 102-profile-cache#T1.3: Implement ProfileMetadata and ProfileLRUCache (RQ-001, RQ-010)
type ProfileMetadata struct {
	ID            string
	Slug          string
	TenantID      string
	Name          string
	Description   *string
	Detectors     []entity.Detector
	Preprocessors []preprocessor.PreprocessorDef
	Enabled       bool
	Version       int
}

func ProfileMetadataFromProfile(p *entity.Profile, version int) *ProfileMetadata {
	var desc *string
	if d := p.Description(); d != nil {
		desc = d
	}
	return &ProfileMetadata{
		ID:            p.ID().String(),
		Slug:          p.Slug().String(),
		TenantID:      p.TenantID().String(),
		Name:          p.Name(),
		Description:   desc,
		Detectors:     p.Detectors(),
		Preprocessors: p.Preprocessors(),
		Enabled:       p.Enabled(),
		Version:       version,
	}
}

type ProfileLRUCache struct {
	inner *lru.Cache[string, *ProfileMetadata]
}

func NewProfileLRUCache(size int) *ProfileLRUCache {
	c, err := lru.New[string, *ProfileMetadata](size)
	if err != nil {
		panic("lru new: " + err.Error())
	}
	return &ProfileLRUCache{inner: c}
}

func (c *ProfileLRUCache) Get(key string) (*ProfileMetadata, bool) { return c.inner.Get(key) }
func (c *ProfileLRUCache) Add(key string, m *ProfileMetadata)      { c.inner.Add(key, m) }
func (c *ProfileLRUCache) Remove(key string)                       { c.inner.Remove(key) }
func (c *ProfileLRUCache) Contains(key string) bool                 { return c.inner.Contains(key) }
func (c *ProfileLRUCache) Len() int                                 { return c.inner.Len() }
func (c *ProfileLRUCache) Purge()                                   { c.inner.Purge() }

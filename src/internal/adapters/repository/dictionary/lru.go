package dictionaryrepo

import (
	"github.com/hashicorp/golang-lru/v2"

	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/preprocessor"
)

// @sk-task 102-profile-cache#T1.3: Implement DictionaryMetadata and DictionaryLRUCache (RQ-001, RQ-010)
// @sk-task tenant-profile-sync#T4.1: Rename ProfileMetadata/ProfileLRUCache (AC-006)
type DictionaryMetadata struct {
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

func DictionaryMetadataFromProfile(p *entity.Profile, version int) *DictionaryMetadata {
	var desc *string
	if d := p.Description(); d != nil {
		desc = d
	}
	return &DictionaryMetadata{
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

type DictionaryLRUCache struct {
	inner *lru.Cache[string, *DictionaryMetadata]
}

func NewDictionaryLRUCache(size int) *DictionaryLRUCache {
	c, err := lru.New[string, *DictionaryMetadata](size)
	if err != nil {
		panic("lru new: " + err.Error())
	}
	return &DictionaryLRUCache{inner: c}
}

func (c *DictionaryLRUCache) Get(key string) (*DictionaryMetadata, bool) { return c.inner.Get(key) }
func (c *DictionaryLRUCache) Add(key string, m *DictionaryMetadata)      { c.inner.Add(key, m) }
func (c *DictionaryLRUCache) Remove(key string)                       { c.inner.Remove(key) }
func (c *DictionaryLRUCache) Contains(key string) bool                 { return c.inner.Contains(key) }
func (c *DictionaryLRUCache) Len() int                                 { return c.inner.Len() }
func (c *DictionaryLRUCache) Purge()                                   { c.inner.Purge() }
